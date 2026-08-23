package utilities

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type PathConfig struct {
	Servers []string `json:"servers"`
}

type MCPManager struct {
	paths    []string
	sessions []*mcp.ClientSession
	mu       sync.RWMutex
	config   PathConfig
}

func NewMCPManager() *MCPManager {
	m := &MCPManager{}
	m.LoadPaths()
	return m
}

func configPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return homeDir + "/.veronica/mcps.json", nil
}

func (m *MCPManager) LoadPaths() {
	filePath, err := configPath()
	if err != nil {
		fmt.Printf("Warning: Failed to resolve home directory: %v\n", err)
		return
	}
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		m.config = PathConfig{Servers: []string{}}
		_ = m.saveConfig()
		return
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Warning: Failed to read config: %v\n", err)
		return
	}
	if err := json.Unmarshal(data, &m.config); err != nil {
		fmt.Printf("Warning: Failed to parse config: %v\n", err)
		return
	}
	m.paths = m.config.Servers
}

func (m *MCPManager) AddPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.paths {
		if p == path {
			return nil
		}
	}
	m.paths = append(m.paths, path)
	return m.saveConfig()
}

func (m *MCPManager) RemovePath(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var newPaths []string
	found := false
	for _, p := range m.paths {
		if p == path {
			found = true
			continue
		}
		newPaths = append(newPaths, p)
	}
	if !found {
		return fmt.Errorf("path not found: %s", path)
	}
	m.paths = newPaths
	return m.saveConfig()
}

func (m *MCPManager) saveConfig() error {
	m.config = PathConfig{Servers: m.paths}
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}
	filePath, err := configPath()
	if err != nil {
		return err
	}
	dir := filePath[:strings.LastIndex(filePath, "/")]
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

func (m *MCPManager) InitializeServers() {
	m.mu.Lock()
	m.sessions = nil
	m.mu.Unlock()

	ctx := context.Background()
	var wg sync.WaitGroup
	var mu sync.Mutex
	connected := 0
	failed := 0

	for _, path := range m.paths {
		wg.Add(1)
		go func(serverPath string) {
			defer wg.Done()

			session, err := connectServer(ctx, serverPath)
			if err != nil {
				fmt.Printf("Failed: %s → %v\n", serverPath, err)
				mu.Lock()
				failed++
				mu.Unlock()
				return
			}

			m.mu.Lock()
			m.sessions = append(m.sessions, session)
			m.mu.Unlock()

			mu.Lock()
			connected++
			mu.Unlock()
			fmt.Printf("Connected: %s\n", serverPath)
		}(path)
	}

	wg.Wait()
	fmt.Printf("MCP Init Done → %d connected, %d failed\n", connected, failed)
}

func connectServer(ctx context.Context, serverPath string) (*mcp.ClientSession, error) {
	newClient := func() *mcp.Client {
		return mcp.NewClient(
			&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"},
			nil,
		)
	}

	if strings.HasPrefix(serverPath, "http://") || strings.HasPrefix(serverPath, "https://") {
		streamable := &mcp.StreamableClientTransport{Endpoint: serverPath}
		session, err := newClient().Connect(ctx, streamable, nil)
		if err == nil {
			return session, nil
		}
		streamableErr := err

		sse := &mcp.SSEClientTransport{Endpoint: serverPath}
		session, err = newClient().Connect(ctx, sse, nil)
		if err == nil {
			return session, nil
		}
		return nil, fmt.Errorf("streamable HTTP: %v; SSE fallback: %v", streamableErr, err)
	}

	cmd := exec.Command(serverPath)
	transport := &mcp.CommandTransport{Command: cmd}
	return newClient().Connect(ctx, transport, nil)
}

func (m *MCPManager) GetTools() []mcp.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ctx := context.Background()
	var all []mcp.Tool
	for _, s := range m.sessions {
		res, err := s.ListTools(ctx, nil)
		if err != nil {
			fmt.Println("Tool list error:", err)
			continue
		}
		for _, t := range res.Tools {
			if t != nil {
				all = append(all, *t)
			}
		}
	}
	return all
}

func (m *MCPManager) ExecuteTool(name string, args map[string]any) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.sessions) == 0 {
		return "", fmt.Errorf("no MCP servers connected")
	}
	ctx := context.Background()
	for _, s := range m.sessions {
		res, err := s.CallTool(ctx, &mcp.CallToolParams{
			Name:      name,
			Arguments: args,
		})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				continue
			}
			return "", err
		}
		for _, c := range res.Content {
			if txt, ok := c.(*mcp.TextContent); ok {
				return txt.Text, nil
			}
		}
	}
	return "", fmt.Errorf("tool not found: %s", name)
}

func (m *MCPManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		_ = s.Close()
	}
	m.sessions = nil
}

func (m *MCPManager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions) > 0
}

func (m *MCPManager) GetServerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

func (m *MCPManager) ListPaths() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.paths...)
}
