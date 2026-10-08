package format

import "opencode-reasoning-extractor/internal/model"

// Manifest describes an extraction run and enables resume.
type Manifest struct {
	Version     string         `json:"version"`
	GeneratedAt string         `json:"generated_at"`
	Input       string         `json:"input"`
	CatalogHash string         `json:"catalog_hash"`
	Strict      bool           `json:"strict_openai"`
	Options     map[string]any `json:"options,omitempty"`
	Sessions    int            `json:"sessions_exported"`
	Exported    []string       `json:"exported_session_ids"`
	Topics      map[string]int `json:"topics"`
	Models      map[string]int `json:"models"`
	Agents      map[string]int `json:"agents"`
	// Tools is the static built-in tool registry (schemas are not persisted by
	// OpenCode, so they are shipped here once per run).
	Tools []model.ToolSchema `json:"tools,omitempty"`
}

// RLEntry is a per-session reward/quality record used to filter data for later
// reinforcement learning stages.
type RLEntry struct {
	ID       string             `json:"id"`
	Topic    []string           `json:"topic"`
	Scores   map[string]float64 `json:"topic_scores,omitempty"`
	Agent    string             `json:"agent,omitempty"`
	Model    string             `json:"model,omitempty"`
	Provider string             `json:"provider,omitempty"`
	Subagent bool               `json:"subagent"`
	ParentID string             `json:"parent_id,omitempty"`
	Project  string             `json:"project,omitempty"`
	Reward   Reward             `json:"reward"`
}

// Reward holds derived, verifier-agnostic signals.
type Reward struct {
	ToolCalls       int            `json:"tool_calls"`
	ToolErrors      int            `json:"tool_errors"`
	ToolSuccessRate float64        `json:"tool_success_rate"`
	FinishReasons   map[string]int `json:"finish_reasons"`
	Patches         int            `json:"patches"`
	FilesTouched    int            `json:"files_touched"`
	Tokens          model.Tokens   `json:"tokens"`
	Cost            float64        `json:"cost"`
	HadError        bool           `json:"had_error"`
	FinalFinish     string         `json:"final_finish,omitempty"`
	QualityFlags    []string       `json:"quality_flags,omitempty"`
	Redactions      int            `json:"redactions"`
}
