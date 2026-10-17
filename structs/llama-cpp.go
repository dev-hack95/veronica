// Package structs defines the shared API types and Bubble Tea message
// types used across Veronica.
package structs

import (
	"context"
	"sync"
	"time"
)

// Message is one chat message in the conversation sent to the LLM.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
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

// SegmentKind is the kind of one ordered piece of assistant output.
type SegmentKind string

const (
	SegmentReasoning SegmentKind = "reasoning"
	SegmentTool      SegmentKind = "tool"
	SegmentAnswer    SegmentKind = "answer"
)

// ToolBlock is one MCP tool call: what was requested, how long it took, and
// (when expanded) what it returned.
type ToolBlock struct {
	Name    string
	Args    string
	Running bool
	Elapsed time.Duration
	Output  string
	Err     string
}

// Segment is one piece of assistant output for the turn, kept in
// chronological order: reasoning text, a tool call block, or answer text.
type Segment struct {
	Kind SegmentKind
	Text string // reasoning or answer text
	Tool *ToolBlock
}

// HarnessChunkMsg is emitted by the harness poller with the accumulated state
// of the current turn (ordered output segments, active tool, timings).
type HarnessChunkMsg struct {
	Segments     []Segment
	ActiveTool   string
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
	Segments     []Segment
	Done         bool
	LimitReached bool
	Err          error
	Timings      *Timings
	NewMessages  []Message
	Ctx          context.Context
}

// AppendText appends a streamed chunk to the last segment of the same kind,
// or starts a new segment.
func (s *HarnessState) AppendText(kind SegmentKind, chunk string) {
	if chunk == "" {
		return
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if n := len(s.Segments); n > 0 && s.Segments[n-1].Kind == kind {
		s.Segments[n-1].Text += chunk
		return
	}
	s.Segments = append(s.Segments, Segment{Kind: kind, Text: chunk})
}

// AppendTool starts a new running tool segment and returns its index.
func (s *HarnessState) AppendTool(name, args string) int {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	idx := len(s.Segments)
	s.Segments = append(s.Segments, Segment{Kind: SegmentTool, Tool: &ToolBlock{Name: name, Args: args, Running: true}})
	return idx
}

// FinishTool marks a tool segment complete with its result.
func (s *HarnessState) FinishTool(idx int, elapsed time.Duration, output string, err error) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if idx < 0 || idx >= len(s.Segments) || s.Segments[idx].Tool == nil {
		return
	}
	t := s.Segments[idx].Tool
	t.Running = false
	t.Elapsed = elapsed
	if err != nil {
		t.Err = err.Error()
	} else {
		t.Output = output
	}
}

// Snapshot returns a deep copy of the segments plus the name of the tool
// currently running ("" if none).
func (s *HarnessState) Snapshot() (segments []Segment, activeTool string) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	segments = make([]Segment, len(s.Segments))
	for i, seg := range s.Segments {
		if seg.Tool != nil {
			t := *seg.Tool
			seg.Tool = &t
			if t.Running {
				activeTool = t.Name
			}
		}
		segments[i] = seg
	}
	return segments, activeTool
}
