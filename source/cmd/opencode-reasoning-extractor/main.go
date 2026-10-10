// Command opencode-reasoning-extractor converts opencode session databases into
// topic-partitioned, OpenAI-compatible JSONL datasets for SFT and later RL.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"opencode-reasoning-extractor/internal/pipeline"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("opencode-reasoning-extractor", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `usage: opencode-reasoning-extractor [flags] <opencode-data-dir> <output-dir>

Extracts opencode session histories into topic-partitioned OpenAI-compatible
JSONL datasets. Reads only the relational session/message/part tables.

flags:
`)
		fs.PrintDefaults()
	}

	opts := pipeline.Options{}
	fs.StringVar(&opts.Catalog, "catalog", "", "path to a custom keyword catalog YAML (default: embedded)")
	fs.BoolVar(&opts.Strict, "strict-openai", false, "emit strict {\"messages\":[...]} records without reasoning or metadata")
	fs.StringVar(&opts.Subagents, "subagents", "both", "subagent handling: both|separate|inline|none")
	fs.StringVar(&opts.Models, "models", "", "comma-separated model id substrings to include")
	fs.StringVar(&opts.Agents, "agents", "", "comma-separated agent names to include (exact)")
	fs.StringVar(&opts.Session, "session", "", "export only a single session id")
	fs.IntVar(&opts.Limit, "limit", 0, "maximum number of sessions to export (0 = all)")
	fs.Int64Var(&opts.MinReason, "min-reasoning-tokens", 0, "skip sessions with fewer reasoning tokens")
	fs.BoolVar(&opts.Resume, "resume", false, "resume a previous run, skipping already exported sessions")
	fs.BoolVar(&opts.Force, "force", false, "allow writing into an output directory that already has a manifest")
	fs.BoolVar(&opts.NoRedact, "no-redact", false, "disable secret redaction")
	fs.StringVar(&opts.SystemPrompt, "system-prompt", "none", "system message: none|synth|<file>")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "classify and report without writing output")
	fs.BoolVar(&opts.Verbose, "verbose", false, "print per-session progress")
	fs.BoolVar(&opts.NoSessions, "no-sessions", false, "skip per-session records")
	fs.BoolVar(&opts.NoTurns, "no-turns", false, "skip per-turn records (avoids large context duplication)")
	fs.IntVar(&opts.TurnContext, "turn-context", 0, "max messages of context per turn record (0 = full)")
	fs.IntVar(&opts.MaxTopics, "max-topics", 0, "maximum topics to assign per session (0 = all)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 2 {
		fs.Usage()
		return fmt.Errorf("expected <opencode-data-dir> and <output-dir>")
	}
	opts.Input = rest[0]
	opts.Output = rest[1]

	stats, err := pipeline.Run(opts)
	if err != nil {
		return err
	}

	mode := "extracted"
	if opts.DryRun {
		mode = "dry-run"
	}
	fmt.Printf("%s %d sessions (%d subagents), %d tool calls (%d errors), %d redactions, %d refusals skipped\n",
		mode, stats.Sessions, stats.Subagents, stats.ToolCalls, stats.ToolErrors, stats.Redactions, stats.Refusals)
	printCounts("topics", stats.Topics)
	printCounts("models", stats.Models)
	printCounts("agents", stats.Agents)
	return nil
}

func printCounts(label string, m map[string]int) {
	if len(m) == 0 {
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("  %s:", label)
	for _, k := range keys {
		fmt.Printf(" %s=%d", k, m[k])
	}
	fmt.Println()
}
