package app

import (
	"fmt"
	"veronica/structs"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) runCmd(cmd string) (tea.Model, tea.Cmd) {
	switch cmd {
	case "/new":
		m.isProcessing = false
		m.textInput.Focus()
		m.message = []string{}
		m.memory = []structs.Message{}
		m.historyIdx = -1

		m.message = append(m.message, infoStyle.Render("New Session"))
		m.viewPort.SetContent(m.updateViewportContent())
		m.viewPort.GotoBottom()
		m.textInput.Reset()
		return m, nil

	case "/tools":
		output := m.getAllToolsFormatted()
		m.message = append(m.message, output)
		m.textInput.Reset()
		m.viewPort.SetContent(m.updateViewportContent())
		m.viewPort.GotoBottom()
		return m, nil

	case "/mcp":
		output := m.getMCPStatus()
		m.message = append(m.message, output)
		m.textInput.Reset()
		m.viewPort.SetContent(m.updateViewportContent())
		m.viewPort.GotoBottom()
		return m, nil

	case "/stream":
		m.isStreaming = !m.isStreaming
		m.message = append(m.message, infoStyle.Render(fmt.Sprintf("Mode Switched, Streaming: %v", m.isStreaming)))
		m.textInput.Reset()
		m.viewPort.SetContent(m.updateViewportContent())
		m.viewPort.GotoBottom()
		return m, nil

	case "/bye":
		return m, tea.Quit

	default:
		return m, nil
	}
}
