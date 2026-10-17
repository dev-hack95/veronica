package app

import (
	"fmt"
	"strings"

	"veronica/structs"

	"github.com/charmbracelet/lipgloss"
)

const innerHeight = 1
const footerHeight = 2

var (
	propmtStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	infoStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	thinkingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	mcpStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("111")).Bold(true)
)

func (m *Model) View() string {
	if !m.ready {
		return "Initializing..."
	}
	var s strings.Builder
	s.WriteString(m.viewPort.View() + "\n\n")
	if m.isProcessing {
		s.WriteString(m.spinner.View() + " " + m.phaseLabel() + " (esc: cancel)\n")
	} else {
		modeName := "Standard"
		if m.isStreaming {
			modeName = "Streaming"
		}

		if m.isMCPFormOpen {
			s.WriteString(fmt.Sprintf("%s [Add MCP Server]\n", mcpStyle.Render(">>")))
			s.WriteString(m.mcpInput.View() + "\n")
			s.WriteString(infoStyle.Render("(Enter: Save, Esc: Cancel)"))
		} else {
			s.WriteString(fmt.Sprintf("%s %s\n", infoStyle.Render("["+modeName+"]"), m.textInput.View()))
		}
		toolHint := "show tool output"
		if m.showFullToolOutput {
			toolHint = "hide tool output"
		}
		thinkHint := "show thinking"
		if m.showThinking {
			thinkHint = "hide thinking"
		}
		s.WriteString(infoStyle.Render(fmt.Sprintf("(↑/↓: history, Ctrl+A: MCP Config, Ctrl+O: %s, Ctrl+T: %s, /stream: toggle, ctrl+c: quit)", toolHint, thinkHint)))
	}
	return s.String()
}

// phaseLabel describes what the harness is doing right now for the footer:
// a running tool, active reasoning ("Thinking..."), or answer generation.
func (m *Model) phaseLabel() string {
	if m.activeTool != "" {
		return fmt.Sprintf("Running %s...", m.activeTool)
	}
	if n := len(m.streamingSegments); n > 0 {
		if m.streamingSegments[n-1].Kind == structs.SegmentReasoning {
			return "Thinking..."
		}
		return "Generating..."
	}
	return "Processing..."
}
