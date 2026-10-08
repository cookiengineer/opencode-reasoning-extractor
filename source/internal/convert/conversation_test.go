package convert

import (
	"testing"

	"opencode-reasoning-extractor/internal/model"
)

func TestBuildConversation(t *testing.T) {
	sess := model.Session{ID: "s1", Agent: "build", Model: model.ModelRef{ID: "deepseek-v4-pro"}}
	messages := []model.Message{
		{ID: "m1", Role: "user", TimeCreated: 1},
		{ID: "m2", Role: "assistant", Finish: "tool-calls", TimeCreated: 2},
		{ID: "m3", Role: "assistant", Finish: "stop", TimeCreated: 3},
	}
	parts := []model.Part{
		{ID: "p1", MessageID: "m1", Kind: model.PartText, Text: "Exploit the parser", TimeCreated: 1},
		{ID: "p2", MessageID: "m2", Kind: model.PartReasoning, Text: "Let me look", TimeCreated: 2},
		{ID: "p3", MessageID: "m2", Kind: model.PartTool, Tool: "read", CallID: "call_1",
			State: &model.ToolState{Status: "completed", Input: []byte(`{"filePath":"/p.go"}`), Output: "package p"}, TimeCreated: 3},
		{ID: "p4", MessageID: "m3", Kind: model.PartReasoning, Text: "Found it", TimeCreated: 4},
		{ID: "p5", MessageID: "m3", Kind: model.PartText, Text: "Done", TimeCreated: 5},
	}

	conv := Build(sess, messages, parts, Options{})

	if len(conv.Messages) != 4 {
		t.Fatalf("messages = %d, want 4", len(conv.Messages))
	}
	if conv.Messages[0].Role != "user" || conv.Messages[0].Content != "Exploit the parser" {
		t.Errorf("user message = %+v", conv.Messages[0])
	}
	a := conv.Messages[1]
	if a.Role != "assistant" || a.ReasoningContent != "Let me look" || len(a.ToolCalls) != 1 {
		t.Errorf("assistant = %+v", a)
	}
	if a.ToolCalls[0].Function.Name != "read" || a.ToolCalls[0].Function.Arguments != `{"filePath":"/p.go"}` {
		t.Errorf("tool call = %+v", a.ToolCalls[0])
	}
	if conv.Messages[2].Role != "tool" || conv.Messages[2].ToolCallID != "call_1" || conv.Messages[2].Content != "package p" {
		t.Errorf("tool result = %+v", conv.Messages[2])
	}
	if len(conv.AssistantTurns) != 2 {
		t.Fatalf("assistant turns = %v, want 2", conv.AssistantTurns)
	}
	if conv.Meta.ToolCalls != 1 || conv.Meta.Patches != 0 {
		t.Errorf("meta = %+v", conv.Meta)
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	sess := model.Session{ID: "s1"}
	conv := Build(sess, nil, nil, Options{SystemPrompt: "You are opencode."})
	if len(conv.Messages) != 1 || conv.Messages[0].Role != "system" {
		t.Fatalf("messages = %+v, want single system message", conv.Messages)
	}
	if !conv.Meta.SystemPrompt {
		t.Error("Meta.SystemPrompt = false")
	}
}

func TestTurnRecords(t *testing.T) {
	sess := model.Session{ID: "s1"}
	messages := []model.Message{
		{ID: "m1", Role: "user", TimeCreated: 1},
		{ID: "m2", Role: "assistant", TimeCreated: 2},
		{ID: "m3", Role: "assistant", TimeCreated: 3},
	}
	parts := []model.Part{
		{ID: "p1", MessageID: "m1", Kind: model.PartText, Text: "hi", TimeCreated: 1},
		{ID: "p2", MessageID: "m2", Kind: model.PartText, Text: "a", TimeCreated: 2},
		{ID: "p3", MessageID: "m3", Kind: model.PartText, Text: "b", TimeCreated: 3},
	}
	conv := Build(sess, messages, parts, Options{})
	turns := TurnRecords(conv, []string{"general"}, nil, model.RecordMeta{}, 0)
	if len(turns) != 2 {
		t.Fatalf("turns = %d, want 2", len(turns))
	}
	for _, turn := range turns {
		if turn.Messages[len(turn.Messages)-1].Role != "assistant" {
			t.Errorf("turn %s does not end with assistant", turn.ID)
		}
	}
}
