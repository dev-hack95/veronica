package app

import (
	"context"
	"fmt"
	"strings"
	"veronica/structs"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
)

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.mdRenderer, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(msg.Width-4),
		)
		if !m.ready {
			m.viewPort = viewport.New(m.width, m.height-innerHeight-footerHeight)
			m.ready = true
		} else {
			m.viewPort.Width, m.viewPort.Height = m.width, m.height-innerHeight-footerHeight
		}
		m.viewPort.SetContent(m.updateViewportContent())
		m.textInput.Width = m.width - 2

	case tea.KeyMsg:
		if m.isProcessing {
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
			if msg.Type == tea.KeyEsc {
				m.cancel()
				m.ctx, m.cancel = context.WithCancel(context.Background())
				m.isProcessing = false

				if len(m.memory) > 0 {
					m.memory = m.memory[:len(m.memory)-1]
				}
				if len(m.message) > 0 {
					m.message = m.message[:len(m.message)-1]
				}
				m.message = append(m.message, infoStyle.Render("Request cancelled"))
				m.streamingReasoning = ""
				m.streamingTools = ""
				m.streamingAnswer = ""
				m.historyIdx = -1

				m.viewPort.SetContent(m.updateViewportContent())
				m.viewPort.GotoBottom()
				m.textInput.Focus()
				return m, nil
			}
			return m, nil
		}

		if m.isMCPFormOpen {
			if msg.Type == tea.KeyEsc {
				m.isMCPFormOpen = false
				m.textInput.Focus()
				return m, nil
			}
			if msg.Type == tea.KeyEnter {
				path := m.mcpInput.Value()
				if path != "" {
					if err := m.mcpManager.AddPath(path); err == nil {
						m.message = append(m.message, infoStyle.Render(fmt.Sprintf("MCP Server added: %s", path)))
						m.viewPort.SetContent(m.updateViewportContent())
						m.isMCPFormOpen = false
						m.textInput.Focus()
						return m, nil
					} else {
						m.message = append(m.message, errorStyle.Render(fmt.Sprintf("Error adding path: %v", err)))
						m.viewPort.SetContent(m.updateViewportContent())
					}
				}
			}
			m.mcpInput, cmd = m.mcpInput.Update(msg)
			return m, cmd
		}

		switch msg.Type {
		case tea.KeyEnter:
			prompt := m.textInput.Value()
			if strings.TrimSpace(prompt) == "" {
				return m, nil
			}

			if len(strings.Split(prompt, " ")) == 1 && prompt[0] == '/' {
				return m.runCmd(prompt)
			}

			m.message = append(m.message, propmtStyle.Render(fmt.Sprintf("> %s", prompt)))
			m.prompts = append(m.prompts, prompt)
			m.memory = append(m.memory, structs.Message{Role: "user", Content: prompt})

			m.textInput.Reset()
			m.textInput.Blur()
			m.isProcessing = true
			m.historyIdx = -1

			return m, tea.Batch(m.runHarness(m.memory), spinner.Tick)

		case tea.KeyUp:
			if len(m.prompts) > 0 {
				if m.historyIdx == -1 {
					m.currentPrompt = m.textInput.Value()
					m.historyIdx = len(m.prompts)
				}
				if m.historyIdx > 0 {
					m.historyIdx--
					m.textInput.SetValue(m.prompts[m.historyIdx])
				}
			}
		case tea.KeyDown:
			if m.historyIdx != -1 {
				if m.historyIdx < len(m.prompts)-1 {
					m.historyIdx++
					m.textInput.SetValue(m.prompts[m.historyIdx])
				} else {
					m.historyIdx = -1
					m.textInput.SetValue(m.currentPrompt)
				}
			}

		case tea.KeyCtrlA:
			m.isMCPFormOpen = !m.isMCPFormOpen
			if m.isMCPFormOpen {
				m.mcpInput.Focus()
				m.textInput.Blur()
			} else {
				m.textInput.Focus()
				m.mcpInput.Blur()
			}
			return m, nil

		case tea.KeyCtrlC:
			return m, tea.Quit
		}

	case structs.HarnessChunkMsg:
		if !m.isProcessing {
			return m, nil
		}
		if msg.Err != nil {
			m.isProcessing = false
			m.message = append(m.message, errorStyle.Render(msg.Err.Error()))
			m.viewPort.SetContent(m.updateViewportContent())
			return m, m.textInput.Focus()
		}
		m.streamingReasoning, m.streamingTools, m.streamingAnswer = msg.Reasoning, msg.ToolBlocks, msg.Answer
		m.viewPort.SetContent(m.updateViewportContent())
		m.viewPort.GotoBottom()

		if msg.IsComplete {
			m.isProcessing = false
			final := m.formatAssistantBlock(m.streamingReasoning, m.streamingTools, m.streamingAnswer)
			m.message = append(m.message, final)
			m.memory = append(m.memory, msg.NewMessages...)
			if msg.Timings != nil {
				m.message = append(m.message, infoStyle.Render(fmt.Sprintf("(Completed in %.0fms)", msg.Timings.TimeMS)))
			}
			m.streamingReasoning, m.streamingTools, m.streamingAnswer = "", "", ""
			m.textInput.Focus()
		}
		return m, nil

	case spinner.TickMsg:
		if m.isProcessing {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	if !m.isProcessing {
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.ready {
		m.viewPort, cmd = m.viewPort.Update(msg)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}
