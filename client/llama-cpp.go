package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"veronica/structs"
	"veronica/utilities"
)

type ChatClient struct {
	Model       string  `json:"model"`
	Url         string  `json:"url"`
	Role        string  `json:"role"`
	Stream      bool    `json:"stream"`
	Temperature float32 `json:"temperature"`
	TopP        float32 `json:"top_p"`
	MaxTokens   uint16  `json:"max_tokens"`
}

type Choices struct {
	FinishReason string          `json:"finish_reason"`
	Index        int             `json:"index"`
	Message      structs.Message `json:"message"`
}

type Usage struct {
	CompletionTokens int `json:"completion_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type Response struct {
	ID      string    `json:"id"`
	Choices []Choices `json:"choices"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Object  string    `json:"object"`
	Usage   Usage     `json:"usage"`
}

type payload struct {
	Model       string            `json:"model"`
	Messages    []structs.Message `json:"messages"`
	Tools       []structs.Tool    `json:"tools,omitempty"`
	Temperature float64           `json:"temperature"`
	MaxTokens   int               `json:"max_tokens"`
	TopP        float64           `json:"top_p"`
	Stream      bool              `json:"stream"`
}

type Section struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type Document struct {
	Sections []Section `json:"sections"`
}

type StreamCallbacks struct {
	OnReasoningChunk func(chunk string)
	OnAnswerChunk    func(chunk string)
	OnComplete       func(result *structs.StreamResult)
}

func NewLlamaCppClient(config utilities.ClientConfig) *ChatClient {
	return &ChatClient{
		Model:       config.Model,
		Url:         config.Url,
		Temperature: config.Temperature,
		Stream:      config.Stream,
		TopP:        config.TopP,
		MaxTokens:   config.MaxTokens,
	}
}

func (c *ChatClient) Ping() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	host, port, err := net.SplitHostPort(strings.TrimPrefix(c.Url, "http://"))

	if err != nil {
		return false
	}

	if host == "" || port == "" {
		return false
	}

	addr := net.JoinHostPort(host, port)

	for {
		select {
		case <-ctx.Done():
			return false

		case <-ticker.C:
			dialer := &net.Dialer{Timeout: 2 * time.Second}
			conn, err := dialer.DialContext(ctx, "tcp", addr)
			if err == nil {
				conn.Close()
				return true
			}
		}

	}
}

func (c *ChatClient) Chat(ctx context.Context, data []structs.Message, tools []structs.Tool) (*structs.ChatClientResponse, error) {

	chatpayload := &payload{
		Model:       c.Model,
		Messages:    data,
		Tools:       tools,
		Temperature: float64(c.Temperature),
		Stream:      c.Stream,
		TopP:        float64(c.TopP),
		MaxTokens:   int(c.MaxTokens),
	}

	buff, err := json.Marshal(chatpayload)

	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Url+"/v1/chat/completions", bytes.NewReader(buff))

	if err != nil {
		return nil, err
	}

	res, err := httpClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		return nil, err
	}

	var response structs.ChatClientResponse

	err = json.Unmarshal(body, &response)

	if err != nil {
		return nil, err
	}

	return &response, nil
}

type toolCallAcc struct {
	id       string
	name     string
	args     string
}

func (c *ChatClient) HandleStream(resp *http.Response, callbacks *StreamCallbacks) (*structs.StreamResult, error) {
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	var fullReasoning strings.Builder
	var fullAnswer strings.Builder
	var toolAccs []toolCallAcc
	var result structs.StreamResult

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}

		data := bytes.TrimPrefix(line, []byte("data: "))

		if bytes.Equal(data, []byte("[DONE]")) {
			break
		}

		var chunk structs.StreamResponse
		if err := json.Unmarshal(data, &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 {
			choice := chunk.Choices[0]

			if choice.Delta.ReasoningContent != nil {
				reasoning := *choice.Delta.ReasoningContent
				fullReasoning.WriteString(reasoning)

				if callbacks != nil && callbacks.OnReasoningChunk != nil {
					callbacks.OnReasoningChunk(reasoning)
				}
			}

			if choice.Delta.Content != nil {
				content := *choice.Delta.Content
				fullAnswer.WriteString(content)

				if callbacks != nil && callbacks.OnAnswerChunk != nil {
					callbacks.OnAnswerChunk(content)
				}
			}

			for _, tc := range choice.Delta.ToolCalls {
				for len(toolAccs) <= tc.Index {
					toolAccs = append(toolAccs, toolCallAcc{})
				}
				toolAccs[tc.Index].id += tc.ID
				toolAccs[tc.Index].name += tc.Function.Name
				toolAccs[tc.Index].args += tc.Function.Arguments
			}

			if choice.FinishReason != nil && (*choice.FinishReason == "stop" || *choice.FinishReason == "tool_calls") {
				if chunk.Timings != nil {
					result.Timings = &structs.Timings{
						TokensGenerated: chunk.Timings.PredictedN,
						TimeMS:          chunk.Timings.PredictedMS,
						TokensPerSecond: chunk.Timings.PredictedPerSecond,
					}
				}
				break
			}
		}
	}

	for _, a := range toolAccs {
		if a.name == "" {
			continue
		}
		tc := structs.ToolCall{ID: a.id, Type: "function"}
		tc.Function.Name = a.name
		tc.Function.Arguments = a.args
		result.ToolCalls = append(result.ToolCalls, tc)
	}

	result.Reasoning = fullReasoning.String()
	result.Answer = fullAnswer.String()

	if callbacks != nil && callbacks.OnComplete != nil {
		callbacks.OnComplete(&result)
	}

	return &result, nil
}

func (c *ChatClient) StreamChatWithCallbacks(ctx context.Context, data []structs.Message, tools []structs.Tool, callbacks *StreamCallbacks) (*structs.StreamResult, error) {

	chatpayload := &payload{
		Model:       c.Model,
		Messages:    data,
		Tools:       tools,
		Temperature: float64(c.Temperature),
		Stream:      c.Stream,
		TopP:        float64(c.TopP),
		MaxTokens:   int(c.MaxTokens),
	}

	payloadBytes, err := json.Marshal(chatpayload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.Url+"/v1/chat/completions", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("bad status: " + resp.Status)
	}

	return c.HandleStream(resp, callbacks)
}
