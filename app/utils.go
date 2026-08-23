package app

import (
	"fmt"
	"strings"
	"time"
	"veronica/client"
	"veronica/structs"

	tea "github.com/charmbracelet/bubbletea"
)

const pollInterval = 50 * time.Millisecond

func (m *Model) prompResponse(data []structs.Message) tea.Cmd {

	return func() tea.Msg {
		start := time.Now()
		response, err := m.client.Chat(m.ctx, data)
		if err != nil {
			return structs.ChatResponse{Err: err, Elapsed: time.Since(start)}
		}
		return structs.ChatResponse{Content: *response, Err: nil, Elapsed: time.Since(start)}
	}
}

func (m *Model) promptResponseStream(data []structs.Message) tea.Cmd {
	state := &structs.StreamState{FirstChunk: true, Ctx: m.ctx}
	go func() {
		callbacks := &client.StreamCallbacks{
			OnReasoningChunk: func(chunk string) {
				state.Mu.Lock()
				state.ReasoningBuffer.WriteString(chunk)
				state.Mu.Unlock()
			},
			OnAnswerChunk: func(chunk string) {
				state.Mu.Lock()
				state.AnswerBuffer.WriteString(chunk)
				state.Mu.Unlock()
			},
			OnComplete: func(result *structs.StreamResult) {
				state.Mu.Lock()
				state.Done = true
				state.Timings = result.Timings
				state.Mu.Unlock()
			},
		}
		_, err := m.client.StreamChatWithCallbacks(m.ctx, data, callbacks)
		if err != nil {
			state.Mu.Lock()
			state.Err = err
			state.Done = true
			state.Mu.Unlock()
		}
	}()
	return m.pollStreamState(state)
}

func (m *Model) pollStreamState(state *structs.StreamState) tea.Cmd {
	return func() tea.Msg {
		select {
		case <-time.After(pollInterval):
		case <-state.Ctx.Done():
			state.Mu.Lock()
			state.Done = true
			if state.Err == nil {
				state.Err = state.Ctx.Err()
			}
			state.Mu.Unlock()
		}

		state.Mu.Lock()
		defer state.Mu.Unlock()

		msg := structs.StreamChunkMsg{
			ReasoningChunk: state.ReasoningBuffer.String(),
			AnswerChunk:    state.AnswerBuffer.String(),
			IsComplete:     state.Done,
			IsFirstChunk:   state.FirstChunk,
			Timings:        state.Timings,
			Err:            state.Err,
		}
		state.FirstChunk = false
		if !state.Done {
			return tea.Batch(func() tea.Msg { return msg }, m.pollStreamState(state))()
		}
		return msg
	}
}

func (m *Model) renderMarkdown(content string) string {
	rendered, err := m.mdRenderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(rendered)
}

func (m *Model) formatAssistantBlock(reasoning, answer string) string {
	header := headerStyle.Render("Assistant: ")
	var fullContent strings.Builder

	if reasoning != "" {
		reasoningHeader := thinkingStyle.Render("--- Thinking ---")
		renderedReasoning := m.renderMarkdown(reasoning)
		fullContent.WriteString(reasoningHeader + "\n" + thinkingStyle.Render(renderedReasoning) + "\n\n")
	}

	if answer != "" {
		if reasoning != "" {
			fullContent.WriteString(thinkingStyle.Render("--- Answer ---") + "\n")
		}
		fullContent.WriteString(m.renderMarkdown(answer))
	}

	return fmt.Sprintf("%s\n%s", header, fullContent.String())
}

func (m *Model) updateViewportContent() string {
	content := strings.Join(m.message, "\n\n")
	if m.isProcessing && (m.streamingReasoning != "" || m.streamingAnswer != "") {
		currentStream := m.formatAssistantBlock(m.streamingReasoning, m.streamingAnswer)
		if content != "" {
			content += "\n\n"
		}
		content += currentStream
	}
	return content
}

func (m *Model) getAllToolsFormatted() string {
	tools := m.mcpManager.GetTools()

	if len(tools) == 0 {
		return infoStyle.Render("No tools available (or no servers connected)")
	}

	var b strings.Builder
	b.WriteString(mcpStyle.Render("Available MCP Tools:\n"))

	for i, t := range tools {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, t.Name))
		if t.Description != "" {
			b.WriteString(infoStyle.Render("   "+t.Description) + "\n")
		}
	}

	return b.String()
}

func (m *Model) getMCPStatus() string {
	count := m.mcpManager.GetServerCount()

	if count == 0 {
		return errorStyle.Render("❌ No MCP servers connected")
	}

	paths := m.mcpManager.ListPaths()

	var b strings.Builder
	b.WriteString(mcpStyle.Render(fmt.Sprintf("✅ MCP Connected (%d servers)\n", count)))

	for i, p := range paths {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, p))
	}

	return b.String()
}
