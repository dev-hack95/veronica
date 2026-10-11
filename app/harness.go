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
)

// runHarness executes the tool-calling loop for one user turn. It runs in a
// goroutine, streaming ordered output segments into state; the poller emits
// HarnessChunkMsg.
func (m *Model) runHarness(data []structs.Message) tea.Cmd {
	state := &structs.HarnessState{Ctx: m.ctx}
	m.harnessState = state
	messages := append([]structs.Message{{Role: "system", Content: prompts.GetMarkdownSystemPrompt()}}, data...)
	startLen := len(messages)
	tools := m.mcpManager.GetOpenAITools()

	go func() {
		for iter := 0; iter < maxToolIterations; iter++ {
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
				idx := state.AppendTool(tc.Function.Name, tc.Function.Arguments)
				messages = append(messages, m.executeToolCall(state, idx, tc))
			}

			if iter == maxToolIterations-1 {
				state.Mu.Lock()
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
// appending live chunks to the turn's ordered segments and returning the
// final result.
func (m *Model) callModel(state *structs.HarnessState, messages []structs.Message, tools []structs.Tool) (*structs.StreamResult, error) {
	if m.isStreaming {
		callbacks := &client.StreamCallbacks{
			OnReasoningChunk: func(chunk string) { state.AppendText(structs.SegmentReasoning, chunk) },
			OnAnswerChunk:    func(chunk string) { state.AppendText(structs.SegmentAnswer, chunk) },
		}
		return m.client.StreamChatWithCallbacks(state.Ctx, messages, tools, callbacks)
	}

	resp, err := m.client.Chat(state.Ctx, messages, tools)
	if err != nil {
		return nil, err
	}
	msg := resp.Choices[0].Message
	state.AppendText(structs.SegmentReasoning, msg.ReasoningContent)
	state.AppendText(structs.SegmentAnswer, msg.Content)

	return &structs.StreamResult{
		Reasoning: msg.ReasoningContent,
		Answer:    msg.Content,
		ToolCalls: msg.ToolCalls,
	}, nil
}

// executeToolCall runs one tool call with a per-tool timeout, records the
// outcome on the matching tool segment, and returns the tool result message.
func (m *Model) executeToolCall(state *structs.HarnessState, idx int, tc structs.ToolCall) structs.Message {
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

	state.FinishTool(idx, elapsed, out, err)

	resultMsg := structs.Message{Role: "tool", ToolCallID: tc.ID}
	if err != nil {
		resultMsg.Content = fmt.Sprintf("Error: %v", err)
	} else {
		resultMsg.Content = out
	}
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

		segments, activeTool := state.Snapshot()
		state.Mu.Lock()
		msg := structs.HarnessChunkMsg{
			Segments:     segments,
			ActiveTool:   activeTool,
			IsComplete:   state.Done,
			LimitReached: state.LimitReached,
			Timings:      state.Timings,
			NewMessages:  state.NewMessages,
			Err:          state.Err,
		}
		state.Mu.Unlock()
		return msg
	}
}
