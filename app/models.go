// Package app implements Veronica's Bubble Tea TUI: model state, key
// handling, the MCP tool-calling harness, and rendering.
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

// viewEntry is one block in the viewport conversation: a pre-rendered raw
// line (prompt, notice) or an assistant turn kept as ordered segments so it
// can be re-rendered (e.g. when tool output is expanded with Ctrl+O).
type viewEntry struct {
	kind     string
	text     string            // raw entries
	segments []structs.Segment // assistant entries
	limit    bool              // iteration limit reached (assistant entries)
}

const (
	entryRaw       = "raw"
	entryAssistant = "assistant"
)

type Model struct {
	textInput          textinput.Model
	viewPort           viewport.Model
	spinner            spinner.Model
	client             *client.ChatClient
	memory             []structs.Message
	entries            []viewEntry
	prompts            []string
	isProcessing       bool
	isStreaming        bool
	ready              bool
	historyIdx         int
	currentPrompt      string
	height             int
	width              int
	mdRenderer         *glamour.TermRenderer
	streamingSegments  []structs.Segment
	activeTool         string
	harnessState       *structs.HarnessState
	showFullToolOutput bool
	showThinking       bool
	cursorVisible      bool

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

	chatClient := client.NewLlamaCppClient(config)
	entries := []viewEntry{}
	if chatClient.Ping() {
		entries = append(entries, viewEntry{kind: entryRaw, text: infoStyle.Render(fmt.Sprintf("Connected to %s", config.URL))})
	} else {
		entries = append(entries, viewEntry{kind: entryRaw, text: errorStyle.Render(fmt.Sprintf("LLM server not reachable at %s", config.URL))})
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Model{
		textInput:     ti,
		mcpInput:      mcpTi,
		mcpManager:    mcpManager,
		spinner:       sp,
		client:        chatClient,
		memory:        []structs.Message{},
		entries:       entries,
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
