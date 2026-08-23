package app

import (
	"context"
	"fmt"
	"os"
	"veronica/client"
	"veronica/structs"
	"veronica/utilities"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type Model struct {
	textInput          textinput.Model
	viewPort           viewport.Model
	spinner            spinner.Model
	client             *client.ChatClient
	memory             []structs.Message
	message            []string
	prompts            []string
	isProcessing       bool
	isStreaming        bool
	ready              bool
	historyIdx         int
	currentPrompt      string
	err                error
	height             int
	width              int
	mdRenderer         *glamour.TermRenderer
	streamingReasoning string
	streamingAnswer    string

	ctx    context.Context
	cancel context.CancelFunc

	mcpManager    *utilities.MCPManager
	mcpInput      textinput.Model
	isMCPFormOpen bool
}

func InitModel() *Model {
	ti := textinput.New()
	ti.Placeholder = "How can I help you today? (type /stream to toggle)"
	ti.Prompt = "> "
	ti.Focus()

	mcpTi := textinput.New()
	mcpTi.Placeholder = "Enter MCP Server Path/URL..."
	mcpTi.Focus()
	mcpTi.Width = 80

	sp := spinner.New()
	sp.Spinner = spinner.Line
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	config, err := utilities.CheckFileExist("/.veronica/model.yaml")

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	mcpManager := utilities.NewMCPManager()
	mcpManager.InitializeServers()

	ctx, cancel := context.WithCancel(context.Background())

	return &Model{
		textInput:     ti,
		mcpInput:      mcpTi,
		mcpManager:    mcpManager,
		spinner:       sp,
		client:        client.NewLlamaCppClient(config),
		memory:        []structs.Message{},
		message:       []string{},
		isProcessing:  false,
		isStreaming:   true,
		ready:         false,
		historyIdx:    -1,
		mdRenderer:    renderer,
		isMCPFormOpen: false,
		ctx:           ctx,
		cancel:        cancel,
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, spinner.Tick, tea.EnterAltScreen)
}
