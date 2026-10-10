package structs

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

type Client struct {
	Url string `json:"url"`
}

type Request struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
	Options  struct {
		Temperature float64 `json:"temperature"`
	} `json:"options"`
}

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ToolCallDelta is one streamed fragment of a tool call. Fragments for the
// same call share Index and must be concatenated (ID, Name, Arguments).
type ToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

// Tool is the OpenAI function-calling tool definition sent in the request.
type Tool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters"`
	} `json:"function"`
}

type SessionData struct {
	Query   string          `json:"query"`
	Respone json.RawMessage `json:"response"`
}

type ChatClientResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Index        int    `json:"index"`
		Message      struct {
			Role             string     `json:"role"`
			ReasoningContent string     `json:"reasoning_content"`
			Content          string     `json:"content"`
			ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
		} `json:"message"`
	} `json:"choices"`
	Created           int    `json:"created"`
	Model             string `json:"model"`
	SystemFingerprint string `json:"system_fingerprint"`
	Object            string `json:"object"`
	Usage             struct {
		CompletionTokens int `json:"completion_tokens"`
		PromptTokens     int `json:"prompt_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	ID      string `json:"id"`
	Timings struct {
		CacheN              int     `json:"cache_n"`
		PromptN             int     `json:"prompt_n"`
		PromptMs            float64 `json:"prompt_ms"`
		PromptPerTokenMs    float64 `json:"prompt_per_token_ms"`
		PromptPerSecond     float64 `json:"prompt_per_second"`
		PredictedN          int     `json:"predicted_n"`
		PredictedMs         float64 `json:"predicted_ms"`
		PredictedPerTokenMs float64 `json:"predicted_per_token_ms"`
		PredictedPerSecond  float64 `json:"predicted_per_second"`
	} `json:"timings"`
}

type StreamResponse struct {
	Choices []struct {
		FinishReason *string `json:"finish_reason"`
		Index        int     `json:"index"`
		Delta        struct {
			Role             *string         `json:"role"`
			ReasoningContent *string         `json:"reasoning_content"`
			Content          *string         `json:"content"`
			ToolCalls        []ToolCallDelta `json:"tool_calls,omitempty"`
		} `json:"delta"`
	} `json:"choices"`
	Created           int64  `json:"created"`
	ID                string `json:"id"`
	Model             string `json:"model"`
	SystemFingerprint string `json:"system_fingerprint"`
	Object            string `json:"object"`
	Timings           *struct {
		CacheN              int     `json:"cache_n"`
		PromptN             int     `json:"prompt_n"`
		PromptMS            float64 `json:"prompt_ms"`
		PromptPerTokenMS    float64 `json:"prompt_per_token_ms"`
		PromptPerSecond     float64 `json:"prompt_per_second"`
		PredictedN          int     `json:"predicted_n"`
		PredictedMS         float64 `json:"predicted_ms"`
		PredictedPerTokenMS float64 `json:"predicted_per_token_ms"`
		PredictedPerSecond  float64 `json:"predicted_per_second"`
	} `json:"timings,omitempty"`
}

type Timings struct {
	TokensGenerated int
	TimeMS          float64
	TokensPerSecond float64
}

type StreamResult struct {
	Reasoning string
	Answer    string
	ToolCalls []ToolCall
	Timings   *Timings
}

type ChatResponse struct {
	Content ChatClientResponse
	Err     error
	Elapsed time.Duration
}

// HarnessChunkMsg is emitted by the harness poller with the accumulated state
// of the current turn (reasoning, rendered tool activity, answer).
type HarnessChunkMsg struct {
	Reasoning    string
	ToolBlocks   string
	Answer       string
	IsComplete   bool
	LimitReached bool
	Timings      *Timings
	// NewMessages are the messages added by this turn (assistant/tool),
	// excluding the prompt that started the turn.
	NewMessages []Message
	Err         error
}

// HarnessState is the shared, mutex-guarded state of one harness turn. The
// harness goroutine writes into it; the poller reads snapshots.
type HarnessState struct {
	Mu           sync.Mutex
	Reasoning    strings.Builder
	ToolBlocks   strings.Builder
	Answer       strings.Builder
	Done         bool
	LimitReached bool
	Err          error
	Timings      *Timings
	NewMessages  []Message
	Ctx          context.Context
}
