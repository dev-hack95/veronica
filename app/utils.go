package app

import (
	"fmt"
	"strings"
	"time"

	"veronica/structs"
)

const pollInterval = 50 * time.Millisecond

// maxArgsDisplay caps how long a tool call's args line stays before eliding.
const maxArgsDisplay = 200

func (m *Model) renderMarkdown(content string) string {
	rendered, err := m.mdRenderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(rendered)
}

// formatAssistantBlock renders one assistant turn from its chronologically
// ordered segments: reasoning, tool call blocks, and answer text appear in
// the order they happened, not grouped by kind.
func (m *Model) formatAssistantBlock(segments []structs.Segment, limitReached bool) string {
	header := headerStyle.Render("Assistant: ")
	var fullContent strings.Builder

	seenThinking := false
	seenTool := false
	seenAnswer := false

	for _, seg := range segments {
		switch seg.Kind {
		case structs.SegmentReasoning:
			if !seenThinking {
				fullContent.WriteString(thinkingStyle.Render("--- Thinking ---") + "\n")
				seenThinking = true
			}
			fullContent.WriteString(thinkingStyle.Render(m.renderMarkdown(seg.Text)) + "\n\n")
		case structs.SegmentTool:
			fullContent.WriteString(m.renderToolBlock(seg.Tool))
			seenTool = true
		case structs.SegmentAnswer:
			if !seenAnswer {
				if seenThinking || seenTool {
					fullContent.WriteString(thinkingStyle.Render("--- Answer ---") + "\n")
				}
				seenAnswer = true
			}
			fullContent.WriteString(m.renderMarkdown(seg.Text) + "\n\n")
		}
	}

	if limitReached {
		fullContent.WriteString(errorStyle.Render("Iteration limit reached") + "\n")
	}

	return fmt.Sprintf("%s\n%s", header, fullContent.String())
}

// renderToolBlock renders one tool call. Collapsed by default (args + status
// line); with m.showFullToolOutput the full output/error text is shown too.
func (m *Model) renderToolBlock(t *structs.ToolBlock) string {
	var b strings.Builder
	b.WriteString(mcpStyle.Render(fmt.Sprintf("■ Tool Call: %s", t.Name)) + "\n")
	if t.Args != "" {
		args := t.Args
		if len(args) > maxArgsDisplay {
			args = args[:maxArgsDisplay] + "…"
		}
		b.WriteString(infoStyle.Render("   args: " + args) + "\n")
	}
	switch {
	case t.Running:
		b.WriteString(m.spinner.View() + infoStyle.Render(" running...") + "\n")
	case t.Err != "":
		b.WriteString(errorStyle.Render(fmt.Sprintf("   ✗ %s (%s)", t.Err, t.Elapsed.Round(time.Millisecond))) + "\n")
	default:
		b.WriteString(infoStyle.Render(fmt.Sprintf("   ✓ done in %s", t.Elapsed.Round(time.Millisecond))) + "\n")
	}
	if m.showFullToolOutput && t.Output != "" {
		b.WriteString(indentToolOutput(t.Output) + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// indentToolOutput indents full tool output under its call block (3 spaces,
// assets/design/tokens.md indent-tool) and dims it with thinkingStyle.
func indentToolOutput(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = "   " + l
	}
	return thinkingStyle.Render(strings.Join(lines, "\n"))
}

func (m *Model) updateViewportContent() string {
	parts := make([]string, 0, len(m.entries)+1)
	for _, e := range m.entries {
		if e.kind == entryAssistant {
			parts = append(parts, m.formatAssistantBlock(e.segments, e.limit))
		} else {
			parts = append(parts, e.text)
		}
	}
	if m.isProcessing && len(m.streamingSegments) > 0 {
		parts = append(parts, m.formatAssistantBlock(m.streamingSegments, false))
	}
	return strings.Join(parts, "\n\n")
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
