package format

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"opencode-reasoning-extractor/internal/model"
)

// Output writes JSONL datasets into a topic-partitioned directory tree.
type Output struct {
	root   string
	strict bool

	mu    sync.Mutex
	files map[string]*jsonlWriter
}

type jsonlWriter struct {
	f   *os.File
	buf *bufio.Writer
	enc *json.Encoder
}

// NewOutput creates the base directory tree.
func NewOutput(root string, strict bool) (*Output, error) {
	for _, sub := range []string{"sft", "subagents", "rl"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &Output{root: root, strict: strict, files: map[string]*jsonlWriter{}}, nil
}

// WriteSession appends a per-session record to <tree>/<topic>/sessions.jsonl.
func (o *Output) WriteSession(tree, topic string, rec model.SessionRecord) error {
	w, err := o.writer(filepath.Join(tree, topic, "sessions.jsonl"))
	if err != nil {
		return err
	}
	return w.encode(o.transformSession(rec))
}

// WriteTurn appends a per-turn record to <tree>/<topic>/turns.jsonl.
func (o *Output) WriteTurn(tree, topic string, rec model.TurnRecord) error {
	w, err := o.writer(filepath.Join(tree, topic, "turns.jsonl"))
	if err != nil {
		return err
	}
	return w.encode(o.transformTurn(rec))
}

// WriteRL appends a reward metadata entry to rl/metadata.jsonl.
func (o *Output) WriteRL(entry RLEntry) error {
	w, err := o.writer(filepath.Join("rl", "metadata.jsonl"))
	if err != nil {
		return err
	}
	return w.encode(entry)
}

// Close flushes and closes all writers.
func (o *Output) Close() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	var firstErr error
	for _, w := range o.files {
		if err := w.buf.Flush(); err != nil && firstErr == nil {
			firstErr = err
		}
		if err := w.f.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	o.files = map[string]*jsonlWriter{}
	return firstErr
}

func (o *Output) writer(rel string) (*jsonlWriter, error) {
	key := rel
	o.mu.Lock()
	defer o.mu.Unlock()
	if w, ok := o.files[key]; ok {
		return w, nil
	}
	full := filepath.Join(o.root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(full, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	w := &jsonlWriter{f: f, buf: bufio.NewWriterSize(f, 1<<20)}
	w.enc = json.NewEncoder(w.buf)
	w.enc.SetEscapeHTML(false)
	o.files[key] = w
	return w, nil
}

func (w *jsonlWriter) encode(v any) error {
	return w.enc.Encode(v)
}

// strictSession is the minimal OpenAI fine-tuning-compatible record.
type strictSession struct {
	Messages []model.ChatMessage `json:"messages"`
}

func (o *Output) transformSession(rec model.SessionRecord) any {
	if !o.strict {
		return rec
	}
	return strictSession{Messages: stripReasoning(rec.Messages)}
}

func (o *Output) transformTurn(rec model.TurnRecord) any {
	if !o.strict {
		return rec
	}
	return strictSession{Messages: stripReasoning(rec.Messages)}
}

func stripReasoning(msgs []model.ChatMessage) []model.ChatMessage {
	out := make([]model.ChatMessage, len(msgs))
	for i, m := range msgs {
		m.ReasoningContent = ""
		m.SourceID = ""
		m.Name = ""
		out[i] = m
	}
	return out
}

// WriteManifest writes manifest.json, overwriting any previous version.
func (o *Output) WriteManifest(m Manifest) error {
	full := filepath.Join(o.root, "manifest.json")
	tmp := full + ".tmp"
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, full)
}

// WriteCatalog copies the catalog actually used next to the manifest.
func (o *Output) WriteCatalog(raw []byte) error {
	return os.WriteFile(filepath.Join(o.root, "catalog.yaml"), raw, 0o644)
}

func (o *Output) String() string { return fmt.Sprintf("output(%s)", o.root) }
