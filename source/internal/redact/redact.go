package redact

import (
	"regexp"
)

// Redactor scrubs likely secrets from free text. It is intentionally
// conservative but errs toward removing anything that looks like a credential.
type Redactor struct {
	patterns []pattern
	total    int
}

type pattern struct {
	name string
	re   *regexp.Regexp
	repl string
}

// New builds a redactor with the built-in credential patterns.
func New() *Redactor {
	return &Redactor{patterns: []pattern{
		{
			name: "private-key",
			re:   regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
			repl: `[REDACTED:PRIVATE_KEY]`,
		},
		{
			name: "jwt",
			re:   regexp.MustCompile(`eyJ[A-Za-z0-9_\-]{6,}\.[A-Za-z0-9_\-]{6,}\.[A-Za-z0-9_\-]{6,}`),
			repl: `[REDACTED:JWT]`,
		},
		{
			name: "bearer",
			re:   regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)[A-Za-z0-9\-._~+/=]+`),
			repl: `${1}[REDACTED:TOKEN]`,
		},
		{
			name: "openai-key",
			re:   regexp.MustCompile(`sk-[A-Za-z0-9_\-]{16,}`),
			repl: `[REDACTED:API_KEY]`,
		},
		{
			name: "github-token",
			re:   regexp.MustCompile(`(gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,})`),
			repl: `[REDACTED:GITHUB_TOKEN]`,
		},
		{
			name: "aws-key",
			re:   regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
			repl: `[REDACTED:AWS_KEY]`,
		},
		{
			name: "slack-token",
			re:   regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),
			repl: `[REDACTED:SLACK_TOKEN]`,
		},
		{
			// Quoted secret literals, e.g. password="hunter2secret".
			name: "secret-quoted",
			re:   regexp.MustCompile(`(?i)((?:api[_-]?key|secret|token|password|passwd|client[_-]?secret|access[_-]?key)\s*[:=]\s*)["'][^"'\n]{6,}["']`),
			repl: `${1}[REDACTED:SECRET]`,
		},
		{
			// Unquoted, high-entropy secret values. Requires 20+ token chars
			// and excludes '.', so code like `Password: cred.Password` is left
			// untouched.
			name: "secret-unquoted",
			re:   regexp.MustCompile(`(?i)((?:api[_-]?key|secret|token|password|passwd|client[_-]?secret|access[_-]?key)\s*[:=]\s*)([A-Za-z0-9+/_-]{20,})`),
			repl: `${1}[REDACTED:SECRET]`,
		},
	}}
}

// Redact replaces likely secrets in s and returns the scrubbed string.
func (r *Redactor) Redact(s string) string {
	if s == "" {
		return s
	}
	for _, p := range r.patterns {
		n := len(p.re.FindAllStringIndex(s, -1))
		if n == 0 {
			continue
		}
		r.total += n
		s = p.re.ReplaceAllString(s, p.repl)
	}
	return s
}

// Total reports how many replacements have been made by this redactor.
func (r *Redactor) Total() int { return r.total }

// Count reports how many patterns were compiled (for diagnostics).
func (r *Redactor) Count() int { return len(r.patterns) }
