package classify

import (
	"reflect"
	"testing"
)

func newDefault(t *testing.T) *Classifier {
	t.Helper()
	c, err := Default()
	if err != nil {
		t.Fatalf("default classifier: %v", err)
	}
	return c
}

func hasTopic(topics []string, want string) bool {
	for _, t := range topics {
		if t == want {
			return true
		}
	}
	return false
}

func TestClassifyExploit(t *testing.T) {
	c := newDefault(t)
	topics, _ := c.Classify(Doc{
		Title:    "Implement CVE-2026-75650 stylesmuggler RCE exploit",
		UserText: "Build a proof of concept that achieves remote code execution via a buffer overflow.",
	})
	if !hasTopic(topics, "exploit-development") {
		t.Fatalf("topics = %v, want exploit-development", topics)
	}
}

func TestClassifySoftware(t *testing.T) {
	c := newDefault(t)
	topics, _ := c.Classify(Doc{
		Title:    "Fix AnswerQuestionsDialog integration in Web UI",
		UserText: "Refactor the frontend module and add a unit test for the API endpoint.",
	})
	if !hasTopic(topics, "software-development") {
		t.Fatalf("topics = %v, want software-development", topics)
	}
	if hasTopic(topics, "exploit-development") {
		t.Fatalf("topics = %v, should not include exploit-development", topics)
	}
}

func TestClassifyCyber(t *testing.T) {
	c := newDefault(t)
	topics, _ := c.Classify(Doc{
		Title:    "Configure nftables firewall and review reconnaissance reports",
		UserText: "Harden the SSH configuration and check the IDS alerts from the CTF.",
	})
	if !hasTopic(topics, "cybersecurity") {
		t.Fatalf("topics = %v, want cybersecurity", topics)
	}
}

func TestClassifyFallback(t *testing.T) {
	c := newDefault(t)
	topics, scores := c.Classify(Doc{Title: "lorem ipsum dolor sit amet"})
	if !reflect.DeepEqual(topics, []string{"general"}) {
		t.Fatalf("topics = %v, want [general]", topics)
	}
	if len(scores) != 0 {
		t.Fatalf("scores = %v, want empty", scores)
	}
}

func TestMultiLabel(t *testing.T) {
	c := newDefault(t)
	topics, _ := c.Classify(Doc{
		Title:    "Cyberdefense-wiki article audit and CTF toolchain",
		UserText: "Refactor the frontend code and fix the unit test while auditing the firewall and reconnaissance articles.",
	})
	if len(topics) < 2 {
		t.Fatalf("topics = %v, want multiple", topics)
	}
	if !hasTopic(topics, "cybersecurity") || !hasTopic(topics, "software-development") {
		t.Fatalf("topics = %v, want cybersecurity and software-development", topics)
	}
}

func TestKeywordBoundaries(t *testing.T) {
	re, err := compileKeyword("rce")
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString("RCE via parser") {
		t.Error("rce should match RCE")
	}
	if re.MatchString("source code") {
		t.Error("rce should not match source")
	}
	re2, err := compileKeyword("cve-")
	if err != nil {
		t.Fatal(err)
	}
	if !re2.MatchString("CVE-2026-1234") {
		t.Error("cve- should match CVE-2026-1234")
	}
}
