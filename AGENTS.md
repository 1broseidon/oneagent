# oneagent

Config-driven multi-agent CLI. One interface for Claude, Codex, OpenCode, Pi, and any future agent.

## What this is

A Go library and CLI that wrap AI agent CLIs behind one normalized interface. Built-in backend defaults ship for Claude, Codex, OpenCode, and Pi, with optional overrides in `~/.config/oneagent/backends.json`. New backends are added by config, not by code.

## Repo: https://github.com/1broseidon/oneagent

## Architecture

- `oneagent.go` — core runtime. Builds backend commands, executes them, normalizes JSON and JSONL output, and emits streaming events.
- `config.go` — backend config compiler. Turns the compact JSON config schema into canonical `Backend` structs.
- `thread.go` — portable thread storage, native-session reuse, cross-backend replay, and thread compaction.
- `cmd/oa/main.go` — thin CLI wrapper. Supports normal JSON output plus `--stream` JSONL output.
- Template variables: `{prompt}`, `{model}`, `{cwd}`, `{session}` — substituted into command arrays. Empty variables drop themselves and their preceding flag.
- Two output formats: `json` (single blob) and `jsonl` (line-delimited events with configurable activity/delta/result/session/error selectors).
- Match conditions support dot-paths and `&` for AND: `type=message_update&assistantMessageEvent.type=text_delta`
- Streaming output is intentionally small and normalized: `session`, optional `activity`, optional `delta`, then `done` or `error`.
- Activity templates can interpolate JSON fields with `{...}` placeholders, including array indexes like `{message.content.0.name}`.

## Quality

- `go test ./...` clean
- All functions pass `gocyclo -over 10`
- `go vet` clean
- `gofmt` clean

## What's next

- Consider: `oa init` command to scaffold the config
- Consider: `--text` flag for plain text output (pipe-friendly, no JSON wrapping)
- Consider: broader `activity` coverage for more backends as needed
