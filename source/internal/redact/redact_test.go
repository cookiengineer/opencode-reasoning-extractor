package redact

import (
	"strings"
	"testing"
)

func TestRedactTruePositives(t *testing.T) {
	r := New()
	cases := map[string]string{
		`key = sk-abcdefghijklmnopqrstuvwxyz012345`:    `API_KEY`,
		`Authorization: Bearer abcDEF123.xyz-token`:    `TOKEN`,
		`aws_key=AKIAIOSFODNN7EXAMPLE`:                 `AWS_KEY`,
		`token="ghp_abcdefghijklmnopqrstuvwxyz0123"`:   `SECRET`,
		`password: "hunter2hunter2"`:                   `SECRET`,
		`api_key=0123456789abcdefghijklmnopqrstuvwxyz`: `SECRET`,
	}
	for in, wantMarker := range cases {
		out := r.Redact(in)
		if out == in {
			t.Errorf("Redact(%q) unchanged, expected redaction with %s", in, wantMarker)
		}
		if !strings.Contains(out, "[REDACTED:") {
			t.Errorf("Redact(%q) = %q, missing marker", in, out)
		}
	}
}

func TestRedactFalsePositives(t *testing.T) {
	r := New()
	// Realistic code and prose that must not be mangled.
	cases := []string{
		`credential := &Credential{Username: cred.Username, Password: cred.Password}`,
		`Username: user.username, Password: user.password,`,
		`if token := strings.TrimSpace(line); token != "" {`,
		`The parser reads each token from the buffer.`,
		`secret := config.LoadSecret()`,
		`/work/exocomp/source/parser.go`,
	}
	for _, in := range cases {
		if out := r.Redact(in); out != in {
			t.Errorf("Redact(%q) = %q, expected unchanged", in, out)
		}
	}
}
