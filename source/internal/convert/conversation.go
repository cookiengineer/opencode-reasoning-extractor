package convert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"opencode-reasoning-extractor/internal/model"
)

// ToolOutputResolver returns the full content for a truncated tool output file
// reference. The bool reports whether the content was found.
type ToolOutputResolver func(outputPath string) (string, bool)

// Options controls conversation building.
type Options struct {
	// SystemPrompt, when non-empty, is prepended as a system message describing
	// the agent. Empty means no system message is emitted.
	SystemPrompt string
	// SubstituteTruncated replaces truncated tool output with the full file
	// content when a resolver can find it.
	SubstituteTruncated bool
	Resolver            ToolOutputResolver
}

// Build converts a session's messages and parts into an OpenAI-compatible
// conversation. Messages and parts must already be sorted by time.
func Build(sess model.Session, messages []model.Message, parts []model.Part, opts Options) model.Conversation {
	byMessage := make(map[string][]model.Part)
	for _, p := range parts {
		byMessage[p.MessageID] = append(byMessage[p.MessageID], p)
	}

	conv := model.Conversation{SessionID: sess.ID}
	conv.Meta = model.ConversationMeta{
		Agent:         sess.Agent,
		ModelID:       sess.Model.ID,
		ProviderID:    sess.Model.ProviderID,
		ParentID:      sess.ParentID,
		ProjectID:     sess.ProjectID,
		Title:         sess.Title,
		Directory:     sess.Directory,
		Version:       sess.Version,
		TimeCreated:   sess.TimeCreated,
		TimeUpdated:   sess.TimeUpdated,
		Tokens:        sess.Tokens,
		Cost:          sess.Cost,
		FinishReasons: map[string]int{},
	}

	if opts.SystemPrompt != "" {
		conv.Messages = append(conv.Messages, model.ChatMessage{
			Role:    "system",
			Content: opts.SystemPrompt,
		})
		conv.Meta.SystemPrompt = true
	}

	sorted := append([]model.Message(nil), messages...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].TimeCreated != sorted[j].TimeCreated {
			return sorted[i].TimeCreated < sorted[j].TimeCreated
		}
		return sorted[i].ID < sorted[j].ID
	})

	for _, msg := range sorted {
		msgParts := byMessage[msg.ID]
		sort.SliceStable(msgParts, func(i, j int) bool {
			if msgParts[i].TimeCreated != msgParts[j].TimeCreated {
				return msgParts[i].TimeCreated < msgParts[j].TimeCreated
			}
			return msgParts[i].ID < msgParts[j].ID
		})

		switch msg.Role {
		case "user":
			conv.Messages = append(conv.Messages, model.ChatMessage{
				Role:    "user",
				Content: joinTexts(msgParts, model.PartText),
			})
			conv.Meta.UserTurns++

		case "assistant":
			conv.Meta.FinishReasons[normalizeFinish(msg.Finish)]++
			idx := len(conv.Messages)
			assistant := model.ChatMessage{
				Role:             "assistant",
				Content:          joinTexts(msgParts, model.PartText),
				ReasoningContent: joinTexts(msgParts, model.PartReasoning),
				SourceID:         msg.ID,
			}
			var toolResults []model.ChatMessage
			for _, p := range msgParts {
				switch p.Kind {
				case model.PartTool:
					assistant.ToolCalls = append(assistant.ToolCalls, buildToolCall(p))
					toolResults = append(toolResults, buildToolResult(p, opts))
					conv.Meta.ToolCalls++
					if p.State != nil && p.State.Status == "error" {
						conv.Meta.ToolErrors++
					}
				case model.PartPatch:
					conv.Meta.Patches++
					conv.Meta.FilesTouched += len(p.Files)
				}
			}
			if assistant.Content == "" && len(assistant.ToolCalls) == 0 && assistant.ReasoningContent == "" {
				continue
			}
			if assistant.ReasoningContent != "" {
				conv.Meta.ReasoningTurns++
			}
			conv.Messages = append(conv.Messages, assistant)
			conv.AssistantTurns = append(conv.AssistantTurns, idx)
			conv.Messages = append(conv.Messages, toolResults...)
		}
	}

	return conv
}

func joinTexts(parts []model.Part, kind model.PartKind) string {
	var chunks []string
	for _, p := range parts {
		if p.Kind == kind && p.Text != "" {
			chunks = append(chunks, p.Text)
		}
	}
	return strings.Join(chunks, "\n\n")
}

func normalizeFinish(f string) string {
	if f == "" {
		return "unknown"
	}
	return f
}

func buildToolCall(p model.Part) model.ToolCall {
	args := "{}"
	if p.State != nil && len(p.State.Input) > 0 {
		args = compactJSON(p.State.Input)
	}
	return model.ToolCall{
		ID:   p.CallID,
		Type: "function",
		Function: model.ToolCallFunction{
			Name:      p.Tool,
			Arguments: args,
		},
	}
}

func buildToolResult(p model.Part, opts Options) model.ChatMessage {
	output := ""
	if p.State != nil {
		output = p.State.Output
		if opts.SubstituteTruncated && p.State.Metadata.Truncated && p.State.Metadata.OutputPath != "" && opts.Resolver != nil {
			if full, ok := opts.Resolver(p.State.Metadata.OutputPath); ok {
				output = full
			}
		}
		if output == "" {
			output = p.State.Title
		}
	}
	return model.ChatMessage{
		Role:       "tool",
		ToolCallID: p.CallID,
		Name:       p.Tool,
		Content:    output,
	}
}

func compactJSON(raw json.RawMessage) string {
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		return string(raw)
	}
	return buf.String()
}

// TurnRecords produces one record per assistant turn. Context includes every
// message up to and including the assistant turn; tool results that follow the
// turn are intentionally excluded so the target is a single model response.
// maxContext, when > 0, trims the context to the most recent maxContext
// messages.
func TurnRecords(conv model.Conversation, topics []string, scores map[string]float64, base model.RecordMeta, maxContext int) []model.TurnRecord {
	var out []model.TurnRecord
	for _, idx := range conv.AssistantTurns {
		start := 0
		if maxContext > 0 && idx+1 > maxContext {
			start = idx + 1 - maxContext
		}
		ctx := make([]model.ChatMessage, idx+1-start)
		copy(ctx, conv.Messages[start:idx+1])
		meta := base
		meta.TopicScores = scores
		turn := conv.Messages[idx].SourceID
		if turn == "" {
			turn = fmt.Sprintf("assistant-%d", idx)
		}
		out = append(out, model.TurnRecord{
			ID:       conv.SessionID + ":" + turn,
			Session:  conv.SessionID,
			Topic:    topics,
			Messages: ctx,
			Meta:     meta,
		})
	}
	return out
}
