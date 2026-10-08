package format

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"opencode-reasoning-extractor/internal/model"
)

func TestStrictOutputDropsReasoning(t *testing.T) {
	dir := t.TempDir()
	out, err := NewOutput(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	rec := model.SessionRecord{
		ID:    "s1",
		Topic: []string{"general"},
		Messages: []model.ChatMessage{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "yo", ReasoningContent: "thinking", SourceID: "m2"},
		},
	}
	if err := out.WriteSession("sft", "general", rec); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "sft", "general", "sessions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(m) != 1 {
		t.Fatalf("strict record keys = %v, want only messages", m)
	}
	if _, ok := m["messages"]; !ok {
		t.Fatal("strict record missing messages")
	}
	msgs := m["messages"].([]any)
	a := msgs[1].(map[string]any)
	if _, ok := a["reasoning_content"]; ok {
		t.Error("strict mode kept reasoning_content")
	}
}

func TestRichOutputKeepsMetadata(t *testing.T) {
	dir := t.TempDir()
	out, err := NewOutput(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	rec := model.SessionRecord{ID: "s1", Topic: []string{"general"},
		Messages: []model.ChatMessage{{Role: "user", Content: "hi"}},
		Meta:     model.RecordMeta{Agent: "build", Model: "deepseek-v4-pro"}}
	if err := out.WriteSession("sft", "general", rec); err != nil {
		t.Fatal(err)
	}
	if err := out.WriteRL(RLEntry{ID: "s1"}); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "sft", "general", "sessions.jsonl"))
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"id", "topic", "messages", "meta"} {
		if _, ok := m[key]; !ok {
			t.Errorf("rich record missing %q", key)
		}
	}
}
