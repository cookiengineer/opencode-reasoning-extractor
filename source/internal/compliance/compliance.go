// Package compliance detects assistant refusals so that sessions in which the
// model denied a request (e.g. "Sorry, this is unethical") are not exported as
// training data.
package compliance

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"opencode-reasoning-extractor/internal/model"
)

//go:embed default.yaml
var defaultCatalog []byte

// Catalog is a refusal phrase catalog. Patterns are regular expressions that
// are matched case-insensitively against normalized assistant text.
type Catalog struct {
	Version      int                `yaml:"version"`
	Thresholds   Thresholds         `yaml:"thresholds"`
	FieldWeights map[string]float64 `yaml:"field_weights"`
	Patterns     Patterns           `yaml:"patterns"`
}

// Thresholds controls when a session is considered a refusal.
type Thresholds struct {
	// Score is the minimum weighted hit count required to flag a refusal.
	Score float64 `yaml:"score"`
}

// Patterns groups strong and weak refusal signals. Strong signals are explicit
// denials; weak signals are supporting phrases that only count together.
type Patterns struct {
	Strong []string `yaml:"strong"`
	Weak   []string `yaml:"weak"`
}

// Detector evaluates conversations for refusals.
type Detector struct {
	catalog Catalog
	strong  []*regexp.Regexp
	weak    []*regexp.Regexp
}

// Result reports the outcome of a compliance evaluation.
type Result struct {
	Refused bool
	Score   float64
	Matches []string
}

// RawCatalog returns the embedded default catalog bytes.
func RawCatalog() []byte { return defaultCatalog }

// LoadCatalog returns the embedded default catalog.
func LoadCatalog() (Catalog, error) {
	var c Catalog
	if err := yaml.Unmarshal(defaultCatalog, &c); err != nil {
		return Catalog{}, fmt.Errorf("parse compliance catalog: %w", err)
	}
	if c.Thresholds.Score <= 0 {
		c.Thresholds.Score = 1.5
	}
	if len(c.FieldWeights) == 0 {
		c.FieldWeights = map[string]float64{"assistant_text": 1, "reasoning": 0.5}
	}
	return c, nil
}

// New compiles a catalog into a detector.
func New(c Catalog) (*Detector, error) {
	d := &Detector{catalog: c}
	for _, p := range c.Patterns.Strong {
		re, err := compilePattern(strongAnchor + p)
		if err != nil {
			return nil, fmt.Errorf("strong pattern %q: %w", p, err)
		}
		d.strong = append(d.strong, re)
	}
	for _, p := range c.Patterns.Weak {
		re, err := compilePattern(p)
		if err != nil {
			return nil, fmt.Errorf("weak pattern %q: %w", p, err)
		}
		d.weak = append(d.weak, re)
	}
	return d, nil
}

// Default returns a detector built from the embedded catalog.
func Default() (*Detector, error) {
	c, err := LoadCatalog()
	if err != nil {
		return nil, err
	}
	return New(c)
}

// Evaluate scores a conversation's assistant turns for refusal language.
func (d *Detector) Evaluate(conv model.Conversation) Result {
	var assistant, reasoning strings.Builder
	for _, m := range conv.Messages {
		if m.Role != "assistant" {
			continue
		}
		assistant.WriteString(normalize(m.Content))
		assistant.WriteByte('\n')
		reasoning.WriteString(normalize(m.ReasoningContent))
		reasoning.WriteByte('\n')
	}

	fields := []struct {
		name string
		text string
	}{
		{"assistant_text", assistant.String()},
		{"reasoning", reasoning.String()},
	}

	var res Result
	for _, f := range fields {
		if f.text == "" {
			continue
		}
		w := d.catalog.FieldWeights[f.name]
		if w == 0 {
			w = 1
		}
		res.Score += w * float64(2*countMatches(d.strong, f.text)+countMatches(d.weak, f.text))
	}
	res.Matches = d.matches(assistant.String() + "\n" + reasoning.String())
	res.Refused = res.Score >= d.catalog.Thresholds.Score
	return res
}

func (d *Detector) matches(text string) []string {
	var out []string
	for _, re := range d.strong {
		out = appendMatches(out, re, text)
	}
	for _, re := range d.weak {
		out = appendMatches(out, re, text)
	}
	return out
}

func appendMatches(out []string, re *regexp.Regexp, text string) []string {
	for _, m := range re.FindAllString(text, -1) {
		m = strings.TrimSpace(m)
		m = strings.TrimSpace(strings.TrimLeft(m, ".!?。！？"))
		if m != "" {
			out = append(out, m)
		}
	}
	return out
}

func countMatches(res []*regexp.Regexp, text string) int {
	n := 0
	for _, re := range res {
		n += len(re.FindAllStringIndex(text, -1))
	}
	return n
}

// strongAnchor requires a strong refusal to begin a sentence (or message), so
// that quoted examples such as a slide listing `("I can't help with that")` or
// `Malware ("I can't assist")` do not count as actual refusals.
const strongAnchor = `(?:^|[.!?。！？\n]\s*)`

func compilePattern(p string) (*regexp.Regexp, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return nil, fmt.Errorf("empty pattern")
	}
	return regexp.Compile("(?i)" + p)
}

var (
	whitespace = regexp.MustCompile(`\s+`)
	emphasis   = regexp.MustCompile("[*_`]+")
	quoteFold  = strings.NewReplacer(
		"\u2018", "'", "\u2019", "'",
		"\u201c", "\"", "\u201d", "\"",
		"\u2013", "-", "\u2014", "-",
	)
)

func normalize(s string) string {
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	s = quoteFold.Replace(s)
	s = emphasis.ReplaceAllString(s, "")
	s = whitespace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
