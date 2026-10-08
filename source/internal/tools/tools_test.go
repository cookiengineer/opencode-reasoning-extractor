package tools

import (
	"encoding/json"
	"testing"
)

func TestRegistry(t *testing.T) {
	all := All()
	if len(all) != 11 {
		t.Fatalf("registry has %d tools, want 11", len(all))
	}
	want := []string{"bash", "edit", "glob", "grep", "question", "read", "skill", "task", "todowrite", "webfetch", "write"}
	for _, name := range want {
		got := For([]string{name})
		if len(got) != 1 || got[0].Name != name {
			t.Fatalf("For(%q) = %+v", name, got)
		}
		if got[0].Description == "" {
			t.Errorf("%s: empty description", name)
		}
		var schema map[string]any
		if err := json.Unmarshal(got[0].Parameters, &schema); err != nil {
			t.Errorf("%s: invalid parameters json: %v", name, err)
		}
		if schema["type"] != "object" {
			t.Errorf("%s: parameters type = %v", name, schema["type"])
		}
	}
}

func TestForOrderAndUnknown(t *testing.T) {
	got := For([]string{"read", "nope", "read", "bash"})
	if len(got) != 2 || got[0].Name != "read" || got[1].Name != "bash" {
		t.Fatalf("For = %+v", got)
	}
}

func TestRequiredFields(t *testing.T) {
	bash := For([]string{"bash"})[0]
	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(bash.Parameters, &schema); err != nil {
		t.Fatal(err)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "command" {
		t.Fatalf("bash required = %v", schema.Required)
	}
}
