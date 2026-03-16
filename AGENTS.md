# oneagent

Config-driven multi-agent CLI. One interface for Claude, Codex, OpenCode, Pi, and any future agent.

## What this is

A Go library and CLI that wrap AI agent CLIs behind one normalized interface. Built-in backend defaults ship for Claude, Codex, OpenCode, and Pi, with optional overrides in `~/.config/oneagent/backends.json`. New backends are added by config, not by code.

## Repo: https://github.com/1broseidon/oneagent

## Docs

- `README.md` — landing page and CLI quick start
- `docs/library.md` — detailed Go embedding guide
- `docs/config.md` — backend config schema and examples
- `docs/examples/consumer.md` — concrete consumer example

## Architecture

- `oneagent.go` — core runtime. Defines `Client`, builds backend commands, executes them, normalizes JSON and JSONL output, and emits streaming events.
- `config.go` — backend config compiler. Turns the compact JSON config schema into canonical `Backend` structs.
- `thread.go` — portable thread storage, native-session reuse, cross-backend replay, thread compaction, and the pluggable thread-store interface/default filesystem store.
- `cmd/oa/main.go` — thin CLI wrapper. Defaults to plain text output, with `--json` for final machine output and `--stream --json` or `--jsonl` for normalized JSONL events.
- Template variables: `{prompt}`, `{model}`, `{cwd}`, `{session}` — substituted into command arrays. Empty variables drop themselves and their preceding flag.
- Two output formats: `json` (single blob) and `jsonl` (line-delimited events with configurable activity/delta/result/session/error selectors).
- Match conditions support dot-paths and `&` for AND: `type=message_update&assistantMessageEvent.type=text_delta`
- The normalized stream is intentionally small: `session`, optional `activity`, optional `delta`, then `done` or `error`.
- Activity templates can interpolate JSON fields with `{...}` placeholders, including array indexes like `{message.content.0.name}`.
- Package-level helpers remain backward-compatible wrappers over a default `Client` and filesystem-backed thread store.

## Quality

- `go test ./...` clean
- All functions pass `gocyclo -over 10`
- `go vet` clean
- `gofmt` clean

## What's next

- Consider: `oa init` command to scaffold the config
- Consider: broader `activity` coverage for more backends as needed
