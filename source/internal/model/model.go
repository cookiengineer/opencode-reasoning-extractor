package model

import "encoding/json"

// Tokens mirrors the token accounting stored on sessions and step-finish parts.
type Tokens struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	Reasoning  int64 `json:"reasoning"`
	CacheRead  int64 `json:"cache_read"`
	CacheWrite int64 `json:"cache_write"`
}

// ModelRef identifies the model that produced a session or message.
type ModelRef struct {
	ID         string `json:"id"`
	ProviderID string `json:"providerID"`
	Variant    string `json:"variant"`
}

// Session is one opencode session. ParentID is non-empty for subagent sessions.
type Session struct {
	ID               string
	ProjectID        string
	WorkspaceID      string
	ParentID         string
	Slug             string
	Directory        string
	Path             string
	Title            string
	Version          string
	Agent            string
	Model            ModelRef
	Cost             float64
	Tokens           Tokens
	SummaryAdditions int64
	SummaryDeletions int64
	SummaryFiles     int64
	TimeCreated      int64
	TimeUpdated      int64
}

// Project is the worktree a session belongs to.
type Project struct {
	ID          string
	Worktree    string
	Name        string
	VCS         string
	TimeCreated int64
	TimeUpdated int64
}

// Message is a single user or assistant message row.
type Message struct {
	ID          string
	SessionID   string
	Role        string
	Agent       string
	Mode        string
	ModelID     string
	ProviderID  string
	Cost        float64
	Tokens      Tokens
	Finish      string
	ParentID    string
	TimeCreated int64
	TimeUpdated int64
}

// PartKind enumerates the part types stored by opencode.
type PartKind string

const (
	PartReasoning  PartKind = "reasoning"
	PartText       PartKind = "text"
	PartTool       PartKind = "tool"
	PartStepStart  PartKind = "step-start"
	PartStepFinish PartKind = "step-finish"
	PartPatch      PartKind = "patch"
)

// ToolState is the execution state of a tool call part.
type ToolState struct {
	Status   string          `json:"status"`
	Input    json.RawMessage `json:"input"`
	Output   string          `json:"output"`
	Title    string          `json:"title"`
	Metadata ToolMetadata    `json:"metadata"`
}

// ToolMetadata carries truncation information for large tool outputs.
type ToolMetadata struct {
	Truncated  bool   `json:"truncated"`
	OutputPath string `json:"outputPath"`
	Preview    string `json:"preview"`
}

// Part is a single message part row.
type Part struct {
	ID          string
	MessageID   string
	SessionID   string
	TimeCreated int64
	TimeUpdated int64

	Kind   PartKind
	Text   string
	Tool   string
	CallID string
	State  *ToolState
	Files  []string
	Hash   string
	Reason string
	Tokens Tokens
}

// rawPart is the JSON payload stored in part.data.
type rawPart struct {
	Type   string    `json:"type"`
	Text   string    `json:"text"`
	Tool   string    `json:"tool"`
	CallID string    `json:"callID"`
	State  *rawState `json:"state"`
	Files  []string  `json:"files"`
	Hash   string    `json:"hash"`
	Reason string    `json:"reason"`
	Tokens *Tokens   `json:"tokens"`
}

type rawState struct {
	Status   string          `json:"status"`
	Input    json.RawMessage `json:"input"`
	Output   string          `json:"output"`
	Title    string          `json:"title"`
	Metadata ToolMetadata    `json:"metadata"`
}

// ParsePart decodes a part.data JSON blob into a Part.
func ParsePart(id, messageID, sessionID string, timeCreated, timeUpdated int64, data []byte) (Part, error) {
	var rp rawPart
	if err := json.Unmarshal(data, &rp); err != nil {
		return Part{}, err
	}
	p := Part{
		ID:          id,
		MessageID:   messageID,
		SessionID:   sessionID,
		TimeCreated: timeCreated,
		TimeUpdated: timeUpdated,
		Kind:        PartKind(rp.Type),
		Text:        rp.Text,
		Tool:        rp.Tool,
		CallID:      rp.CallID,
		Files:       rp.Files,
		Hash:        rp.Hash,
		Reason:      rp.Reason,
	}
	if rp.Tokens != nil {
		p.Tokens = *rp.Tokens
	}
	if rp.State != nil {
		p.State = &ToolState{
			Status:   rp.State.Status,
			Input:    rp.State.Input,
			Output:   rp.State.Output,
			Title:    rp.State.Title,
			Metadata: rp.State.Metadata,
		}
	}
	return p, nil
}

// rawMessage is the JSON payload stored in message.data.
type rawMessage struct {
	ParentID   string  `json:"parentID"`
	Role       string  `json:"role"`
	Mode       string  `json:"mode"`
	Agent      string  `json:"agent"`
	ModelID    string  `json:"modelID"`
	ProviderID string  `json:"providerID"`
	Cost       float64 `json:"cost"`
	Finish     string  `json:"finish"`
	Tokens     *Tokens `json:"tokens"`
}

// ParseMessage decodes a message.data JSON blob into a Message.
func ParseMessage(id, sessionID string, timeCreated, timeUpdated int64, data []byte) (Message, error) {
	var rm rawMessage
	if err := json.Unmarshal(data, &rm); err != nil {
		return Message{}, err
	}
	m := Message{
		ID:          id,
		SessionID:   sessionID,
		Role:        rm.Role,
		Agent:       rm.Agent,
		Mode:        rm.Mode,
		ModelID:     rm.ModelID,
		ProviderID:  rm.ProviderID,
		Cost:        rm.Cost,
		Finish:      rm.Finish,
		ParentID:    rm.ParentID,
		TimeCreated: timeCreated,
		TimeUpdated: timeUpdated,
	}
	if rm.Tokens != nil {
		m.Tokens = *rm.Tokens
	}
	return m, nil
}
