package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"opencode-reasoning-extractor/internal/format"
	"opencode-reasoning-extractor/internal/model"
	"opencode-reasoning-extractor/internal/testutil"
)

func TestRunExtraction(t *testing.T) {
	input := testutil.FixtureDir(t)
	out := filepath.Join(t.TempDir(), "dataset")

	stats, err := Run(Options{
		Input:     input,
		Output:    out,
		Subagents: "both",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Sessions != 2 {
		t.Fatalf("sessions = %d, want 2", stats.Sessions)
	}
	if stats.Subagents != 1 {
		t.Fatalf("subagents = %d, want 1", stats.Subagents)
	}

	// Root session should be classified as exploit-development and land under sft.
	rootFile := filepath.Join(out, "sft", "exploit-development", "sessions.jsonl")
	recs := readSessions(t, rootFile)
	if len(recs) != 1 {
		t.Fatalf("root session records = %d, want 1", len(recs))
	}
	rec := recs[0]
	if rec.ID != "ses_root" {
		t.Errorf("id = %q", rec.ID)
	}
	assertToolPairing(t, rec.Messages)
	if rec.Meta.Agent != "build" {
		t.Errorf("agent = %q", rec.Meta.Agent)
	}
	foundRead := false
	for _, ts := range rec.Meta.Tools {
		if ts.Name == "read" {
			foundRead = true
		}
	}
	if !foundRead {
		t.Errorf("meta.tools missing the used tool: %+v", rec.Meta.Tools)
	}

	// No raw secret must survive.
	raw, err := os.ReadFile(rootFile)
	if err != nil {
		t.Fatal(err)
	}
	if contains(string(raw), "supersecretvalue123") {
		t.Error("secret leaked into output")
	}

	// Subagent should be exported under its own top-level tree.
	subFiles, err := filepath.Glob(filepath.Join(out, "subagents", "*", "sessions.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(subFiles) != 1 {
		t.Fatalf("subagent files = %v, want 1", subFiles)
	}

	// RL metadata and manifest.
	rl := readLines(t, filepath.Join(out, "rl", "metadata.jsonl"))
	if len(rl) != 2 {
		t.Fatalf("rl entries = %d, want 2", len(rl))
	}
	var entry format.RLEntry
	if err := json.Unmarshal([]byte(rl[0]), &entry); err != nil {
		t.Fatalf("rl json: %v", err)
	}
	if entry.ID != "ses_child" && entry.ID != "ses_root" {
		t.Errorf("unexpected rl id %q", entry.ID)
	}

	mf, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(mf, &manifest); err != nil {
		t.Fatalf("manifest json: %v", err)
	}
	if manifest["sessions_exported"].(float64) != 2 {
		t.Errorf("manifest sessions = %v", manifest["sessions_exported"])
	}
	toolsArr, ok := manifest["tools"].([]any)
	if !ok || len(toolsArr) != 11 {
		t.Errorf("manifest tools = %T len=%d, want 11", manifest["tools"], len(toolsArr))
	}
}

func TestRunDryRun(t *testing.T) {
	input := testutil.FixtureDir(t)
	out := filepath.Join(t.TempDir(), "dataset")
	stats, err := Run(Options{Input: input, Output: out, Subagents: "both", DryRun: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if stats.Sessions != 2 {
		t.Fatalf("sessions = %d, want 2", stats.Sessions)
	}
	if _, err := os.Stat(filepath.Join(out, "manifest.json")); !os.IsNotExist(err) {
		t.Error("dry run wrote a manifest")
	}
}

func readSessions(t *testing.T, path string) []model.SessionRecord {
	t.Helper()
	lines := readLines(t, path)
	var out []model.SessionRecord
	for _, l := range lines {
		var r model.SessionRecord
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatalf("session json: %v", err)
		}
		out = append(out, r)
	}
	return out
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out []string
	start := 0
	for i, c := range b {
		if c == '\n' {
			if i > start {
				out = append(out, string(b[start:i]))
			}
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, string(b[start:]))
	}
	return out
}

func assertToolPairing(t *testing.T, msgs []model.ChatMessage) {
	t.Helper()
	pending := map[string]bool{}
	for _, m := range msgs {
		switch m.Role {
		case "assistant":
			for _, tc := range m.ToolCalls {
				pending[tc.ID] = true
			}
		case "tool":
			if !pending[m.ToolCallID] {
				t.Errorf("tool result %q has no matching call", m.ToolCallID)
			}
			delete(pending, m.ToolCallID)
		}
	}
}

func contains(hay, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
