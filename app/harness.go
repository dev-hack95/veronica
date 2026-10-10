package app

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"veronica/app/workflows/prompts"
	"veronica/client"
	"veronica/structs"

	tea "github.com/charmbracelet/bubbletea"
)

// Harness limits (see assets/design/harness.md, Safety & Limits).
const (
	maxToolIterations = 8
	toolTimeout       = 30 * time.Second
	maxArgsDisplay    = 200
)

// runHarness executes the tool-calling loop for one user turn. It runs in a
// goroutine, streaming progress into state; the poller emits HarnessChunkMsg.
func (m *Model) runHarness(data []structs.Message) tea.Cmd {
	state := &structs.HarnessState{Ctx: m.ctx}
	messages := append([]structs.Message{{Role: "system", Content: prompts.GetMarkdownSystemPrompt()}}, data...)
	startLen := len(messages)
	tools := m.mcpManager.GetOpenAITools()

	go func() {
		for iter := 0; iter < maxToolIterations; iter++ {
			if iter > 0 {
				state.Mu.Lock()
				if state.Reasoning.Len() > 0 {
					state.Reasoning.WriteString("\n\n")
				}
				state.Mu.Unlock()
			}

			result, err := m.callModel(state, messages, tools)
			if err != nil {
				state.Mu.Lock()
				state.Err = err
				state.Done = true
				state.Mu.Unlock()
				return
			}

			assistant := structs.Message{Role: "assistant", Content: result.Answer}
			if len(result.ToolCalls) == 0 {
				messages = append(messages, assistant)
				state.Mu.Lock()
				state.NewMessages = messages[startLen:]
				state.Timings = result.Timings
				state.Done = true
				state.Mu.Unlock()
				return
			}
			assistant.ToolCalls = result.ToolCalls
			messages = append(messages, assistant)

			for _, tc := range result.ToolCalls {
				messages = append(messages, m.executeToolCall(state, tc))
			}

			if iter == maxToolIterations-1 {
				state.Mu.Lock()
				state.ToolBlocks.WriteString(errorStyle.Render("Iteration limit reached") + "\n")
				state.LimitReached = true
				state.NewMessages = messages[startLen:]
				state.Done = true
				state.Mu.Unlock()
				return
			}
		}
	}()

	return m.pollHarnessState(state)
}

// callModel makes one chat completion (streamed or not, per m.isStreaming),
// writing live chunks into state and returning the final result.
func (m *Model) callModel(state *structs.HarnessState, messages []structs.Message, tools []structs.Tool) (*structs.StreamResult, error) {
	if m.isStreaming {
		callbacks := &client.StreamCallbacks{
			OnReasoningChunk: func(chunk string) {
				state.Mu.Lock()
				state.Reasoning.WriteString(chunk)
				state.Mu.Unlock()
			},
			OnAnswerChunk: func(chunk string) {
				state.Mu.Lock()
				state.Answer.WriteString(chunk)
				state.Mu.Unlock()
			},
		}
		return m.client.StreamChatWithCallbacks(state.Ctx, messages, tools, callbacks)
	}

	resp, err := m.client.Chat(state.Ctx, messages, tools)
	if err != nil {
		return nil, err
	}
	msg := resp.Choices[0].Message
	state.Mu.Lock()
	state.Reasoning.WriteString(msg.ReasoningContent)
	state.Answer.WriteString(msg.Content)
	state.Mu.Unlock()

	return &structs.StreamResult{
		Reasoning: msg.ReasoningContent,
		Answer:    msg.Content,
		ToolCalls: msg.ToolCalls,
	}, nil
}

// executeToolCall renders the tool-call block, runs the tool with a per-tool
// timeout, renders the outcome, and returns the tool result message.
func (m *Model) executeToolCall(state *structs.HarnessState, tc structs.ToolCall) structs.Message {
	argsDisplay := tc.Function.Arguments
	if len(argsDisplay) > maxArgsDisplay {
		argsDisplay = argsDisplay[:maxArgsDisplay] + "…"
	}

	state.Mu.Lock()
	state.ToolBlocks.WriteString(mcpStyle.Render(fmt.Sprintf("■ Tool Call: %s", tc.Function.Name)) + "\n")
	state.ToolBlocks.WriteString(infoStyle.Render("   args: "+argsDisplay) + "\n")
	state.Mu.Unlock()

	var args map[string]any
	if tc.Function.Arguments != "" {
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			args = nil
		}
	}

	tctx, cancel := context.WithTimeout(state.Ctx, toolTimeout)
	start := time.Now()
	out, err := m.mcpManager.ExecuteToolCtx(tctx, tc.Function.Name, args)
	elapsed := time.Since(start)
	cancel()

	resultMsg := structs.Message{Role: "tool", ToolCallID: tc.ID}

	state.Mu.Lock()
	if err != nil {
		resultMsg.Content = fmt.Sprintf("Error: %v", err)
		state.ToolBlocks.WriteString(errorStyle.Render(fmt.Sprintf("   ✗ %v (%v)", err, elapsed.Round(time.Millisecond))) + "\n\n")
	} else {
		resultMsg.Content = out
		state.ToolBlocks.WriteString(infoStyle.Render(fmt.Sprintf("   ✓ done in %v", elapsed.Round(time.Millisecond))) + "\n\n")
	}
	state.Mu.Unlock()

	return resultMsg
}

func (m *Model) pollHarnessState(state *structs.HarnessState) tea.Cmd {
	return func() tea.Msg {
		select {
		case <-time.After(pollInterval):
		case <-state.Ctx.Done():
			state.Mu.Lock()
			if !state.Done {
				state.Done = true
				if state.Err == nil {
					state.Err = state.Ctx.Err()
				}
			}
			state.Mu.Unlock()
		}

		state.Mu.Lock()
		defer state.Mu.Unlock()

		msg := structs.HarnessChunkMsg{
			Reasoning:    state.Reasoning.String(),
			ToolBlocks:   state.ToolBlocks.String(),
			Answer:       state.Answer.String(),
			IsComplete:   state.Done,
			LimitReached: state.LimitReached,
			Timings:      state.Timings,
			NewMessages:  state.NewMessages,
			Err:          state.Err,
		}
		if !state.Done {
			return tea.Batch(func() tea.Msg { return msg }, m.pollHarnessState(state))()
		}
		return msg
	}
}
