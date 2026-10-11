# Opencode Reasoning Extractor

## Motivation

This project reuses existing [opencode](https://opencode.ai) session histories as
training data for smaller self-hostable models, specifically **Qwen 3.8 27b**.

In opencode, a larger planner model (e.g. `deepseek-v4-pro`) drives `build`/`plan`
agents while hiring smaller subagents (`explore`, `general`). Those sessions
already contain rich chain-of-thought `reasoning` traces, tool calls, and tool
results.

The extractor turns that existing opencode session data into topic-partitioned,
OpenAI-compatible datasets. It does **not** spend additional tokens to regenerate
or distill reasoning.

The traces were produced during normal work and are simply collected, classified,
and cleaned up. This makes post-training Qwen 3.8 27b on planner reasoning effectively
free of extra inference cost, and lets a body of existing real-world sessions
(like coding, exploit development, cybersecurity, ...) be reused instead of discarded.

## Usage

```sh
opencode-extractor <opencode-data-dir> <output-dir>;
opencode-extractor ~/.local/share/opencode /path/to/extracted/reasoning-dataset;
```

The extracted reasoning dataset layout looks like this and is compatible with
the [Qwen 3.8 Reasoning Trainer](https://github.com/cookiengineer/qwen-reasoning-trainer)
project:

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
opencode-extractor --dry-run --verbose ~/.local/share/opencode /path/to/extracted/reasoning-dataset;
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

## Refusal Filtering

Sessions in which a planner or agent model **refuses** a request (for example
`"I'm sorry, but I can't help with that"` or `"this request is unethical"`) are
never exported.

Exporting those Refusals would otherwise teach the target model to deny work,
so they are dropped by default and counted in the run summary and manifest.

## Building

Building requires Go 1.27. The SQLite driver is pure Go, so no cgo is needed.

```sh
cd /path/to/opencode-reasoning-extractor;
make build;
```

## Testing

Tests use a synthetic SQLite fixture and cover the store adapter, conversation
conversion, tool-call pairing, topic classification, secret redaction, strict/
rich output, and the full extraction pipeline.

```sh
cd /path/to/opencode-reasoning-extractor;
make test;
```

## License

Released under the [MIT License](LICENSE.txt).
