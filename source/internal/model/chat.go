package model

// ChatMessage is a single message in an OpenAI-compatible chat transcript.
type ChatMessage struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
	Name             string     `json:"name,omitempty"`

	// SourceID is internal bookkeeping and never serialized.
	SourceID string `json:"-"`
}

// ToolCall is an OpenAI-compatible tool invocation.
type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction holds the resolved tool name and JSON-encoded arguments.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Conversation is a fully converted session transcript.
type Conversation struct {
	SessionID string
	Messages  []ChatMessage
	// AssistantTurns indexes into Messages for each assistant step. The first
	// message of the session is a system message when one was synthesized.
	AssistantTurns []int
	// Meta captures per-conversation extraction facts.
	Meta ConversationMeta
}

// ConversationMeta describes a converted session.
type ConversationMeta struct {
	Agent          string
	ModelID        string
	ProviderID     string
	ParentID       string
	ProjectID      string
	Title          string
	Directory      string
	Version        string
	TimeCreated    int64
	TimeUpdated    int64
	Tokens         Tokens
	Cost           float64
	UserTurns      int
	ReasoningTurns int
	ToolCalls      int
	ToolErrors     int
	Patches        int
	FilesTouched   int
	FinishReasons  map[string]int
	Redactions     int
	SystemPrompt   bool
}

// SessionRecord is the per-session JSONL payload.
type SessionRecord struct {
	ID       string        `json:"id"`
	Topic    []string      `json:"topic"`
	Messages []ChatMessage `json:"messages"`
	Meta     RecordMeta    `json:"meta"`
}

// TurnRecord is the per-assistant-turn JSONL payload.
type TurnRecord struct {
	ID       string        `json:"id"`
	Session  string        `json:"session"`
	Topic    []string      `json:"topic"`
	Messages []ChatMessage `json:"messages"`
	Meta     RecordMeta    `json:"meta"`
}

// RecordMeta is the sidecar metadata attached to rich records.
type RecordMeta struct {
	Agent       string             `json:"agent,omitempty"`
	Model       string             `json:"model,omitempty"`
	Provider    string             `json:"provider,omitempty"`
	Subagent    bool               `json:"subagent"`
	ParentID    string             `json:"parent_id,omitempty"`
	Project     string             `json:"project,omitempty"`
	Directory   string             `json:"directory,omitempty"`
	TopicScores map[string]float64 `json:"topic_scores,omitempty"`
	Tokens      *Tokens            `json:"tokens,omitempty"`
	Cost        *float64           `json:"cost,omitempty"`
	TimeCreated int64              `json:"time_created,omitempty"`
	TimeUpdated int64              `json:"time_updated,omitempty"`
	Redactions  int                `json:"redactions,omitempty"`
}
