package classify

import (
	_ "embed"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed default.yaml
var defaultCatalog []byte

// Catalog is a topic keyword catalog.
type Catalog struct {
	Version      int                `yaml:"version"`
	Fallback     string             `yaml:"fallback"`
	Thresholds   Thresholds         `yaml:"thresholds"`
	FieldWeights map[string]float64 `yaml:"field_weights"`
	Categories   []Category         `yaml:"categories"`
}

// Thresholds controls multi-label selection.
type Thresholds struct {
	MultiLabel float64 `yaml:"multi_label"`
}

// Category is one topic with weighted keyword sets.
type Category struct {
	ID       string   `yaml:"id"`
	Label    string   `yaml:"label"`
	Keywords Keywords `yaml:"keywords"`
}

// Keywords groups strong and weak signal terms.
type Keywords struct {
	Strong []string `yaml:"strong"`
	Weak   []string `yaml:"weak"`
}

// Doc is the text extracted from a session, split by field.
type Doc struct {
	Title         string
	UserText      string
	Reasoning     string
	AssistantText string
	ToolInput     string
	ToolOutput    string
	Paths         string
}

// Classifier assigns topics to session documents.
type Classifier struct {
	catalog  Catalog
	compiled []compiledCategory
}

type compiledCategory struct {
	id     string
	strong []*regexp.Regexp
	weak   []*regexp.Regexp
}

// RawCatalog returns the catalog bytes from disk, or the embedded default when
// path is empty.
func RawCatalog(path string) ([]byte, error) {
	if path == "" {
		return defaultCatalog, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}
	return b, nil
}

// LoadCatalog reads a catalog from disk, or returns the embedded default when
// path is empty.
func LoadCatalog(path string) (Catalog, error) {
	raw := defaultCatalog
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return Catalog{}, fmt.Errorf("read catalog: %w", err)
		}
		raw = b
	}
	var c Catalog
	if err := yaml.Unmarshal(raw, &c); err != nil {
		return Catalog{}, fmt.Errorf("parse catalog: %w", err)
	}
	if c.Fallback == "" {
		c.Fallback = "general"
	}
	if c.Thresholds.MultiLabel <= 0 {
		c.Thresholds.MultiLabel = 0.35
	}
	if len(c.FieldWeights) == 0 {
		c.FieldWeights = map[string]float64{
			"title": 3, "user_text": 2, "reasoning": 1,
			"assistant_text": 1, "tool_input": 1.5, "tool_output": 0.5, "path": 1.5,
		}
	}
	return c, nil
}

// New compiles a catalog into a classifier.
func New(c Catalog) (*Classifier, error) {
	cl := &Classifier{catalog: c}
	for _, cat := range c.Categories {
		cc := compiledCategory{id: cat.ID}
		for _, kw := range cat.Keywords.Strong {
			re, err := compileKeyword(kw)
			if err != nil {
				return nil, fmt.Errorf("category %s keyword %q: %w", cat.ID, kw, err)
			}
			cc.strong = append(cc.strong, re)
		}
		for _, kw := range cat.Keywords.Weak {
			re, err := compileKeyword(kw)
			if err != nil {
				return nil, fmt.Errorf("category %s keyword %q: %w", cat.ID, kw, err)
			}
			cc.weak = append(cc.weak, re)
		}
		cl.compiled = append(cl.compiled, cc)
	}
	return cl, nil
}

// Default returns the classifier built from the embedded catalog.
func Default() (*Classifier, error) {
	c, err := LoadCatalog("")
	if err != nil {
		return nil, err
	}
	return New(c)
}

// Classify returns the selected topics and normalized scores. topics is never
// empty: the catalog fallback is returned when nothing scores above threshold.
func (c *Classifier) Classify(doc Doc) (topics []string, scores map[string]float64) {
	raw := make(map[string]float64, len(c.compiled))
	var max float64
	for _, cc := range c.compiled {
		s := c.score(cc, doc)
		raw[cc.id] = s
		if s > max {
			max = s
		}
	}
	scores = make(map[string]float64, len(raw))
	if max <= 0 {
		return []string{c.catalog.Fallback}, scores
	}
	for id, s := range raw {
		scores[id] = s / max
	}
	type pair struct {
		id string
		s  float64
	}
	var sel []pair
	for _, cc := range c.compiled {
		norm := raw[cc.id] / max
		if norm >= c.catalog.Thresholds.MultiLabel {
			sel = append(sel, pair{cc.id, norm})
		}
	}
	if len(sel) == 0 {
		return []string{c.catalog.Fallback}, scores
	}
	sort.SliceStable(sel, func(i, j int) bool { return sel[i].s > sel[j].s })
	for _, p := range sel {
		topics = append(topics, p.id)
	}
	return topics, scores
}

func (c *Classifier) score(cc compiledCategory, doc Doc) float64 {
	fields := []struct {
		name string
		text string
	}{
		{"title", doc.Title},
		{"user_text", doc.UserText},
		{"reasoning", doc.Reasoning},
		{"assistant_text", doc.AssistantText},
		{"tool_input", doc.ToolInput},
		{"tool_output", doc.ToolOutput},
		{"path", doc.Paths},
	}
	var total float64
	for _, f := range fields {
		if f.text == "" {
			continue
		}
		w := c.catalog.FieldWeights[f.name]
		if w == 0 {
			w = 1
		}
		hits := countMatches(cc.strong, f.text)*2 + countMatches(cc.weak, f.text)
		total += w * float64(hits)
	}
	return total
}

func countMatches(res []*regexp.Regexp, text string) int {
	n := 0
	for _, re := range res {
		n += len(re.FindAllStringIndex(text, -1))
	}
	return n
}

func compileKeyword(kw string) (*regexp.Regexp, error) {
	kw = strings.TrimSpace(kw)
	if kw == "" {
		return nil, fmt.Errorf("empty keyword")
	}
	escaped := regexp.QuoteMeta(strings.ToLower(kw))
	var b strings.Builder
	b.WriteString("(?i)")
	if startsWord(kw) {
		b.WriteString(`(?:^|[^a-z0-9])`)
	}
	b.WriteString(escaped)
	if endsWord(kw) {
		b.WriteString(`(?:$|[^a-z0-9])`)
	}
	return regexp.Compile(b.String())
}

func startsWord(s string) bool {
	return isWordByte(s[0])
}

func endsWord(s string) bool {
	return isWordByte(s[len(s)-1])
}

func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
