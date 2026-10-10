# Opencode Reasoning Extractor

## Motivation

This project reuses existing [opencode](https://opencode.ai) session histories as
training data for smaller self-hostable models, specifically **Qwen 3.8 27b**.

In opencode, a larger planner model (e.g. `deepseek-v4-pro`) drives `build`/`plan`
agents while hiring smaller subagents (`explore`, `general`). Those sessions
already contain rich chain-of-thought `reasoning` traces, tool calls, and tool
results.

The extractor turns that existing data into topic-partitioned, OpenAI-compatible
datasets. It does **not** spend additional tokens to regenerate or distill
reasoning. The traces were produced during normal work and are simply collected,
classified, and cleaned up. This makes post-training Qwen 3.8 27b on planner
reasoning effectively free of extra inference cost, and lets a body of existing
real-world sessions (coding, exploit development, cybersecurity, ...) be reused
instead of discarded.

Sessions that were already run with the target model
(`huihui_ai/Qwen3.8-abliterated:27b`) are exported too, providing on-policy data
for later reinforcement learning stages.

## Usage

```sh
opencode-reasoning-extractor <opencode-data-dir> <output-dir>
opencode-reasoning-extractor ~/.local/share/opencode /path/to/extracted/dataset
```

The input is the opencode local share directory (containing `opencode.db`) or the
database file itself. Topics are detected automatically, so no manual sorting is
needed. Subagent sessions are exported both inline in their parent transcript and
as standalone top-level histories.

Extracted dataset layout:

```
<output-dir>/
  manifest.json                 run metadata and exported session ids
  catalog.yaml                  the keyword catalog used
  refusals.yaml                 the refusal dictionary used
  sft/<topic>/sessions.jsonl    full multi-turn conversations
  sft/<topic>/turns.jsonl       per-assistant-turn samples
  subagents/<topic>/...         standalone subagent histories
  rl/metadata.jsonl             per-session reward/quality signals
```

Inspect before writing:

```sh
opencode-reasoning-extractor --dry-run --verbose ~/.local/share/opencode /path/to/out
```

Useful flags:

| Flag | Description |
| --- | --- |
| `--strict-openai` | emit only `{"messages":[...]}`, dropping `reasoning_content` and metadata |
| `--system-prompt none\|synth\|<file>` | add a system message |
| `--subagents both\|separate\|inline\|none` | subagent export mode |
| `--catalog <file>` | custom keyword catalog YAML |
| `--no-redact` | disable secret scrubbing |
| `--max-topics N` | cap topics per session |
| `--no-turns` / `--turn-context N` | skip or bound per-turn context (full context is large) |
| `--models`, `--agents`, `--session`, `--limit`, `--min-reasoning-tokens` | filters |
| `--resume`, `--force` | append to or overwrite a previous run |

## Refusal filtering

Sessions in which a planner or agent model **refuses** a request (for example
`"I'm sorry, but I can't help with that"` or `"this request is unethical"`) are
never exported. Refusals would otherwise teach the target model to deny work, so
they are dropped by default and counted in the run summary and manifest. There
is no opt-out: compliance is part of the extraction contract.

Matching uses a bundled phrase dictionary (`internal/compliance/default.yaml`),
seeded with the typical denial phrasings produced by DeepSeek v4 Pro and Flash
in both English and Chinese. Patterns are case-insensitive regular expressions
that are scored by field (`assistant_text` weighs more than `reasoning`); strong
explicit denials trip the threshold on their own, while weak supporting phrases
only count together. The dictionary is a curated seed, not an exhaustive list:
refusals are open-ended, so grow `default.yaml` as new phrasings are observed.
The effective dictionary is copied to `refusals.yaml` next to the output for
provenance, and its hash is recorded in `manifest.json`.

## Building

```sh
cd source
go build -o ../build/opencode-reasoning-extractor ./cmd/opencode-reasoning-extractor
```

Requires Go 1.27. The SQLite driver is pure Go, so no cgo is needed.

## Testing

```sh
cd source
go test ./...
```

Tests use a synthetic SQLite fixture and cover the store adapter, conversation
conversion, tool-call pairing, topic classification, secret redaction, strict/
rich output, and the full extraction pipeline.

## License

Released under the [MIT License](LICENSE.txt).
