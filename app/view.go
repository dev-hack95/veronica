package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const innerHeight = 1
const footerHeight = 2

var (
	propmtStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	headerStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true)
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
		footer := "Processing..."
		if m.activeTool != "" {
			footer = fmt.Sprintf("Running %s...", m.activeTool)
		}
		s.WriteString(m.spinner.View() + " " + footer + " (esc: cancel)\n")
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
		s.WriteString(infoStyle.Render(fmt.Sprintf("(↑/↓: history, Ctrl+A: MCP Config, Ctrl+O: %s, /stream: toggle, ctrl+c: quit)", toolHint)))
	}
	return s.String()
}
