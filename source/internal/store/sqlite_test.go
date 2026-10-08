package store

import (
	"testing"

	"opencode-reasoning-extractor/internal/testutil"
)

func TestOpenAndRead(t *testing.T) {
	dbPath := testutil.Fixture(t)
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	projects, err := st.Projects()
	if err != nil {
		t.Fatalf("Projects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("projects = %d, want 1", len(projects))
	}

	sessions, err := st.Sessions()
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions = %d, want 2", len(sessions))
	}
	var child string
	for _, s := range sessions {
		if s.ID == "ses_child" {
			child = s.ParentID
			if s.Model.ID != "deepseek-flash" {
				t.Errorf("child model = %q", s.Model.ID)
			}
		}
	}
	if child != "ses_root" {
		t.Errorf("child parent = %q, want ses_root", child)
	}

	msgs, err := st.Messages("ses_root")
	if err != nil {
		t.Fatalf("Messages: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("messages = %d, want 3", len(msgs))
	}

	parts, err := st.Parts("ses_root")
	if err != nil {
		t.Fatalf("Parts: %v", err)
	}
	if len(parts) != 10 {
		t.Fatalf("parts = %d, want 10", len(parts))
	}
}

func TestOpenMissing(t *testing.T) {
	if _, err := Open("/nonexistent/path"); err == nil {
		t.Fatal("expected error for missing path")
	}
}
