# oneagent

Config-driven multi-agent CLI. One interface for Claude, Codex, Cursor, OpenCode, Cline, Pi, and any future agent.

## What this is

A Go library and CLI that wrap any AI agent CLI behind a unified JSON interface. Built-in backend defaults ship for Claude, Codex, OpenCode, and Pi, with optional overrides in `~/.config/oneagent/backends.json`. Zero code changes are required to add a new backend.

## Repo: https://github.com/1broseidon/oneagent

## Architecture

- `oneagent.go` — the library. Key types: `Backend` (config), `RunOpts` (input), `Response` (output). Key function: `Run()` which builds a command from templates, executes it, parses output per format spec.
- `cmd/oa/main.go` — thin CLI wrapper. Parses flags, calls `oneagent.Run()`, outputs JSON.
- Template variables: `{prompt}`, `{model}`, `{cwd}`, `{session}` — substituted into command arrays. Empty variables drop themselves and their preceding flag.
- Two output formats: `json` (single blob) and `jsonl` (line-delimited events with `result_when`/`session_when`/`error_when` match conditions).
- Match conditions support dot-paths and `&` for AND: `type=message_update&assistantMessageEvent.type=text_delta`
- `result_append: true` accumulates across multiple matching events (for streaming delta backends like pi).

## Quality

- All functions pass `gocyclo -over 10`
- `go vet` clean
- `gofmt` clean

## What's next

- Wire tele (../tele) to use oneagent as a library instead of its own copy of the dispatch logic
- Homebrew tap for `brew install oa`
- Tests cover parsing, thread portability, and CLI flows
- Consider: `oa init` command to scaffold the config
- Consider: `--text` flag for plain text output (pipe-friendly, no JSON wrapping)
