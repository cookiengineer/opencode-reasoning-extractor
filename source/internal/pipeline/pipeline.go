package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"opencode-reasoning-extractor/internal/classify"
	"opencode-reasoning-extractor/internal/convert"
	"opencode-reasoning-extractor/internal/format"
	"opencode-reasoning-extractor/internal/model"
	"opencode-reasoning-extractor/internal/redact"
	"opencode-reasoning-extractor/internal/store"
	"opencode-reasoning-extractor/internal/tools"
)

// Version is the extractor version reported in the manifest.
const Version = "0.1.0"

// Options configures a run.
type Options struct {
	Input        string
	Output       string
	Catalog      string
	Strict       bool
	Subagents    string // both|separate|inline|none
	Models       string
	Agents       string
	Session      string
	Limit        int
	MinReason    int64
	Resume       bool
	Force        bool
	NoRedact     bool
	SystemPrompt string
	DryRun       bool
	Verbose      bool

	NoSessions  bool
	NoTurns     bool
	TurnContext int
	MaxTopics   int
}

// Stats summarizes a run.
type Stats struct {
	Sessions   int
	Subagents  int
	Topics     map[string]int
	Models     map[string]int
	Agents     map[string]int
	ToolCalls  int
	ToolErrors int
	Redactions int
}

// Run performs the extraction.
func Run(opts Options) (*Stats, error) {
	st, err := store.Open(opts.Input)
	if err != nil {
		return nil, err
	}
	defer st.Close()

	projects, err := st.Projects()
	if err != nil {
		return nil, err
	}
	sessions, err := st.Sessions()
	if err != nil {
		return nil, err
	}

	rawCatalog, err := classify.RawCatalog(opts.Catalog)
	if err != nil {
		return nil, err
	}
	cat, err := classify.LoadCatalog(opts.Catalog)
	if err != nil {
		return nil, err
	}
	classifier, err := classify.New(cat)
	if err != nil {
		return nil, err
	}

	stats := &Stats{Topics: map[string]int{}, Models: map[string]int{}, Agents: map[string]int{}}

	if err := prepareOutput(opts); err != nil {
		return nil, err
	}

	var out *format.Output
	if !opts.DryRun {
		out, err = format.NewOutput(opts.Output, opts.Strict)
		if err != nil {
			return nil, err
		}
		defer out.Close()
	}

	exported, err := loadExported(opts)
	if err != nil {
		return nil, err
	}

	exportSub := opts.Subagents != "none" && opts.Subagents != "inline"

	modelFilter := splitFilter(opts.Models)
	agentFilter := splitFilter(opts.Agents)

	processed := 0
	for _, sess := range sessions {
		if !matches(sess, modelFilter, agentFilter, opts.Session) {
			continue
		}
		if opts.MinReason > 0 && sess.Tokens.Reasoning < opts.MinReason {
			continue
		}
		if exported[sess.ID] {
			continue
		}
		if opts.Limit > 0 && processed >= opts.Limit {
			break
		}

		conv, topics, scores, err := analyze(st, sess, opts, classifier)
		if err != nil {
			return nil, fmt.Errorf("session %s: %w", sess.ID, err)
		}
		if len(conv.Messages) == 0 {
			continue
		}
		if opts.MaxTopics > 0 && len(topics) > opts.MaxTopics {
			topics = topics[:opts.MaxTopics]
		}
		processed++

		// Subagents are only exported when requested. Dry runs still report
		// them so the classification summary stays complete.
		if sess.ParentID != "" && !exportSub && !opts.DryRun {
			continue
		}
		tree := "sft"
		if sess.ParentID != "" {
			tree = "subagents"
		}

		stats.Sessions++
		if sess.ParentID != "" {
			stats.Subagents++
		}
		for _, t := range topics {
			stats.Topics[t]++
		}
		stats.Models[sess.Model.ID]++
		stats.Agents[sess.Agent]++
		stats.ToolCalls += conv.Meta.ToolCalls
		stats.ToolErrors += conv.Meta.ToolErrors
		stats.Redactions += conv.Meta.Redactions

		if opts.DryRun {
			exported[sess.ID] = true
			if opts.Verbose {
				fmt.Printf("  %s  agent=%-8s model=%-28s topics=%v\n", sess.ID, sess.Agent, sess.Model.ID, topics)
			}
			continue
		}

		meta := recordMeta(sess, projects, scores, conv.Meta.ToolNames)
		if !opts.NoTurns {
			for _, turn := range convert.TurnRecords(conv, topics, scores, meta, opts.TurnContext) {
				for _, topic := range topics {
					if err := out.WriteTurn(tree, topic, turn); err != nil {
						return nil, err
					}
				}
			}
		}
		if !opts.NoSessions {
			for _, topic := range topics {
				if err := out.WriteSession(tree, topic, model.SessionRecord{
					ID: sess.ID, Topic: topics, Messages: conv.Messages, Meta: meta,
				}); err != nil {
					return nil, err
				}
			}
		}

		if err := out.WriteRL(buildRL(conv, sess, topics, scores)); err != nil {
			return nil, err
		}
		exported[sess.ID] = true

		if opts.Verbose {
			fmt.Printf("  %s -> %s/%s (%d msgs)\n", sess.ID, tree, topics, len(conv.Messages))
		}
	}

	if !opts.DryRun && out != nil {
		if err := out.WriteCatalog(rawCatalog); err != nil {
			return nil, err
		}
		if err := out.WriteManifest(buildManifest(opts, rawCatalog, exported, stats)); err != nil {
			return nil, err
		}
	}
	return stats, nil
}

func prepareOutput(opts Options) error {
	if opts.DryRun {
		return nil
	}
	manifestPath := filepath.Join(opts.Output, "manifest.json")
	if _, err := os.Stat(manifestPath); err == nil && !opts.Resume && !opts.Force {
		return fmt.Errorf("output %s already contains a manifest; use --resume or --force", opts.Output)
	}
	if _, err := os.Stat(opts.Output); os.IsNotExist(err) {
		return os.MkdirAll(opts.Output, 0o755)
	}
	return nil
}

func loadExported(opts Options) (map[string]bool, error) {
	exported := map[string]bool{}
	if !opts.Resume {
		return exported, nil
	}
	b, err := os.ReadFile(filepath.Join(opts.Output, "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return exported, nil
		}
		return nil, err
	}
	var m format.Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	for _, id := range m.Exported {
		exported[id] = true
	}
	return exported, nil
}

func analyze(st store.Store, sess model.Session, opts Options, classifier *classify.Classifier) (model.Conversation, []string, map[string]float64, error) {
	messages, err := st.Messages(sess.ID)
	if err != nil {
		return model.Conversation{}, nil, nil, err
	}
	parts, err := st.Parts(sess.ID)
	if err != nil {
		return model.Conversation{}, nil, nil, err
	}

	conv := convert.Build(sess, messages, parts, convert.Options{
		SystemPrompt: resolveSystemPrompt(opts, sess),
	})

	if !opts.NoRedact {
		r := redact.New()
		for i := range conv.Messages {
			conv.Messages[i].Content = r.Redact(conv.Messages[i].Content)
			conv.Messages[i].ReasoningContent = r.Redact(conv.Messages[i].ReasoningContent)
		}
		conv.Meta.Redactions = r.Total()
	}

	doc := buildDoc(sess, conv)
	topics, scores := classifier.Classify(doc)
	return conv, topics, scores, nil
}

func buildDoc(sess model.Session, conv model.Conversation) classify.Doc {
	var user, reasoning, asst, toolInput, toolOutput strings.Builder
	for _, m := range conv.Messages {
		switch m.Role {
		case "user":
			user.WriteString(m.Content)
			user.WriteString("\n")
		case "assistant":
			reasoning.WriteString(m.ReasoningContent)
			reasoning.WriteString("\n")
			asst.WriteString(m.Content)
			asst.WriteString("\n")
			for _, tc := range m.ToolCalls {
				toolInput.WriteString(tc.Function.Name)
				toolInput.WriteString(" ")
				toolInput.WriteString(tc.Function.Arguments)
				toolInput.WriteString("\n")
			}
		case "tool":
			toolOutput.WriteString(m.Content)
			toolOutput.WriteString("\n")
		}
	}
	return classify.Doc{
		Title:         sess.Title,
		UserText:      user.String(),
		Reasoning:     reasoning.String(),
		AssistantText: asst.String(),
		ToolInput:     toolInput.String(),
		ToolOutput:    toolOutput.String(),
		Paths:         sess.Directory,
	}
}

func recordMeta(sess model.Session, projects map[string]model.Project, scores map[string]float64, toolNames []string) model.RecordMeta {
	meta := model.RecordMeta{
		Agent:       sess.Agent,
		Model:       sess.Model.ID,
		Provider:    sess.Model.ProviderID,
		Subagent:    sess.ParentID != "",
		ParentID:    sess.ParentID,
		Project:     sess.ProjectID,
		Directory:   sess.Directory,
		TopicScores: scores,
		Tokens:      &sess.Tokens,
		TimeCreated: sess.TimeCreated,
		TimeUpdated: sess.TimeUpdated,
		Tools:       tools.For(toolNames),
	}
	cost := sess.Cost
	meta.Cost = &cost
	if p, ok := projects[sess.ProjectID]; ok {
		meta.Project = filepath.Base(p.Worktree)
	}
	return meta
}

func buildRL(conv model.Conversation, sess model.Session, topics []string, scores map[string]float64) format.RLEntry {
	rate := 1.0
	if conv.Meta.ToolCalls > 0 {
		rate = 1 - float64(conv.Meta.ToolErrors)/float64(conv.Meta.ToolCalls)
	}
	var flags []string
	if conv.Meta.ToolErrors > 0 {
		flags = append(flags, "tool_error")
	}
	if conv.Meta.ReasoningTurns > 0 {
		flags = append(flags, "has_reasoning")
	}
	return format.RLEntry{
		ID:       sess.ID,
		Topic:    topics,
		Scores:   scores,
		Agent:    sess.Agent,
		Model:    sess.Model.ID,
		Provider: sess.Model.ProviderID,
		Subagent: sess.ParentID != "",
		ParentID: sess.ParentID,
		Project:  sess.ProjectID,
		Reward: format.Reward{
			ToolCalls:       conv.Meta.ToolCalls,
			ToolErrors:      conv.Meta.ToolErrors,
			ToolSuccessRate: rate,
			FinishReasons:   conv.Meta.FinishReasons,
			Patches:         conv.Meta.Patches,
			FilesTouched:    conv.Meta.FilesTouched,
			Tokens:          sess.Tokens,
			Cost:            sess.Cost,
			HadError:        conv.Meta.ToolErrors > 0,
			QualityFlags:    flags,
			Redactions:      conv.Meta.Redactions,
		},
	}
}

func buildManifest(opts Options, rawCatalog []byte, exported map[string]bool, stats *Stats) format.Manifest {
	ids := make([]string, 0, len(exported))
	for id := range exported {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	sum := sha256.Sum256(rawCatalog)
	return format.Manifest{
		Version:     Version,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Input:       opts.Input,
		CatalogHash: hex.EncodeToString(sum[:]),
		Strict:      opts.Strict,
		Options: map[string]any{
			"subagents": opts.Subagents,
			"models":    opts.Models,
			"agents":    opts.Agents,
			"redact":    !opts.NoRedact,
		},
		Sessions: len(ids),
		Exported: ids,
		Topics:   stats.Topics,
		Models:   stats.Models,
		Agents:   stats.Agents,
		Tools:    tools.All(),
	}
}

func resolveSystemPrompt(opts Options, sess model.Session) string {
	switch opts.SystemPrompt {
	case "", "none":
		return ""
	case "synth":
		agent := sess.Agent
		if agent == "" {
			agent = "build"
		}
		if sess.Directory != "" {
			return fmt.Sprintf("You are opencode's %q agent. Work in %s and use the available tools to complete the task.", agent, sess.Directory)
		}
		return fmt.Sprintf("You are opencode's %q agent. Use the available tools to complete the task.", agent)
	default:
		b, err := os.ReadFile(opts.SystemPrompt)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func splitFilter(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func matches(sess model.Session, models, agents []string, session string) bool {
	if session != "" && sess.ID != session {
		return false
	}
	if len(models) > 0 {
		hay := strings.ToLower(sess.Model.ID)
		ok := false
		for _, m := range models {
			if strings.Contains(hay, m) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(agents) > 0 {
		hay := strings.ToLower(sess.Agent)
		ok := false
		for _, a := range agents {
			if hay == a {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
