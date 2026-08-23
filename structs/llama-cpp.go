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
	Role    string `json:"role"`
	Content string `json:"content"`
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
			Role             string `json:"role"`
			ReasoningContent string `json:"reasoning_content"`
			Content          string `json:"content"`
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
			Role             *string `json:"role"`
			ReasoningContent *string `json:"reasoning_content"`
			Content          *string `json:"content"`
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

type StreamResult struct {
	Reasoning string
	Answer    string
	Timings   *struct {
		TokensGenerated int
		TimeMS          float64
		TokensPerSecond float64
	}
}

type ChatResponse struct {
	Content ChatClientResponse
	Err     error
	Elapsed time.Duration
}

type StreamChunkMsg struct {
	ReasoningChunk string
	AnswerChunk    string
	IsComplete     bool
	IsFirstChunk   bool
	Timings        *struct {
		TokensGenerated int
		TimeMS          float64
		TokensPerSecond float64
	}
	Err error
}

type StreamState struct {
	ReasoningBuffer strings.Builder
	AnswerBuffer    strings.Builder
	Mu              sync.Mutex
	Done            bool
	Err             error
	FirstChunk      bool
	Timings         *struct {
		TokensGenerated int
		TimeMS          float64
		TokensPerSecond float64
	}
	Ctx context.Context
}
