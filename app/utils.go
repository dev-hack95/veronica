package app

import (
	"fmt"
	"strings"
	"time"
)

const pollInterval = 50 * time.Millisecond

func (m *Model) renderMarkdown(content string) string {
	rendered, err := m.mdRenderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(rendered)
}

func (m *Model) formatAssistantBlock(reasoning, toolBlocks, answer string) string {
	header := headerStyle.Render("Assistant: ")
	var fullContent strings.Builder

	if reasoning != "" {
		reasoningHeader := thinkingStyle.Render("--- Thinking ---")
		renderedReasoning := m.renderMarkdown(reasoning)
		fullContent.WriteString(reasoningHeader + "\n" + thinkingStyle.Render(renderedReasoning) + "\n\n")
	}

	if toolBlocks != "" {
		fullContent.WriteString(toolBlocks)
	}

	if answer != "" {
		if reasoning != "" || toolBlocks != "" {
			fullContent.WriteString(thinkingStyle.Render("--- Answer ---") + "\n")
		}
		fullContent.WriteString(m.renderMarkdown(answer))
	}

	return fmt.Sprintf("%s\n%s", header, fullContent.String())
}

func (m *Model) updateViewportContent() string {
	content := strings.Join(m.message, "\n\n")
	if m.isProcessing && (m.streamingReasoning != "" || m.streamingTools != "" || m.streamingAnswer != "") {
		currentStream := m.formatAssistantBlock(m.streamingReasoning, m.streamingTools, m.streamingAnswer)
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
