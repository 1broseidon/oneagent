# oneagent

[![GitHub Stars](https://img.shields.io/github/stars/1broseidon/oneagent?style=social)](https://github.com/1broseidon/oneagent/stargazers)
[![Go Reference](https://pkg.go.dev/badge/github.com/1broseidon/oneagent.svg)](https://pkg.go.dev/github.com/1broseidon/oneagent)
[![Go Report Card](https://goreportcard.com/badge/github.com/1broseidon/oneagent)](https://goreportcard.com/report/github.com/1broseidon/oneagent)

Config-driven multi-agent CLI.

`oa` gives Claude, Codex, OpenCode, Pi, and any future agent CLI a single normalized interface. Every backend gets the same flags, the same JSON and streaming output, and portable conversation threads. Adding a new agent is a JSON config edit — no code required.

## Prerequisites

- At least one supported agent CLI installed and signed in (e.g., `claude`, `codex`, `opencode`, or `pi`)

## Install

Homebrew:

```sh
brew install 1broseidon/tap/oa
```

Or with Go:

```sh
go install github.com/1broseidon/oneagent/cmd/oa@latest
```

## Quick start

```sh
# Talk to Claude (the default backend)
oa "explain this codebase"

# Use a different backend
oa -b codex "fix the auth bug"

# Machine-readable JSON output
oa --json "explain this codebase"

# Live text stream
oa --stream "review the repo"

# Normalized JSONL stream (for piping to other tools)
oa --jsonl "review the repo"

# Portable threads — start on one backend, continue on another
oa -t auth-fix "investigate the failing auth tests"
oa -b codex -t auth-fix "patch the bug"
oa -b claude -t auth-fix "summarize what changed"

# Specify model and working directory
oa -b pi -m "google/gemini-2.5-pro" -C ~/project "add tests"

# Resume a native session
oa -b claude -s abc123 "now refactor it"

# Pipe content as context
git diff | oa -b claude "review these changes"
cat internal/auth/handler.go | oa -b codex "find bugs in this file"
go test ./... 2>&1 | oa -b claude "fix these test failures"

# Thread management
oa thread list
oa thread show auth-fix
oa thread compact auth-fix
```

Works out of the box if `claude`, `codex`, `opencode`, or `pi` is installed and signed in.

## Agent-as-tool

`oa` works as a dispatch layer for agents that want to delegate work to other agents. An outer agent (e.g., Claude Code) can run `oa` as a background task to send a targeted edit to a different model, then inspect the diff when it's done:

```sh
# From inside an agent session — dispatch a file edit to gpt-5.4 via Pi
oa -b pi -m openai-codex/gpt-5.4 "Edit internal/auth/handler.go: add rate limiting to Login" --jsonl

# Verify the result
git diff internal/auth/handler.go
```

The normalized output means the outer agent can parse results from any backend without special handling. Portable threads let you chain follow-ups across models.

An [agent skill](./skills/oa-dispatch/) is included for agents that support the [Agent Skills](https://agentskills.io) format. Install it with:

```sh
npx skills add 1broseidon/oneagent --skill oa-dispatch
```

## Pipelines

Chain agents sequentially with `&&` and shared threads. Each step sees the file changes and conversation context from previous steps:

```sh
# Build → review → document, different agents, one thread
oa -b codex -t feat "add input validation to the signup handler" && \
oa -b codex -t feat "review the changes, run tests, report any issues" && \
git diff | oa -b claude "write a changelog entry for this change"
```

Pipe content into any step as context:

```sh
# Feed test output to an agent for diagnosis
go test ./... 2>&1 | oa -b claude -t fix "diagnose these failures and fix them"

# Code review a specific diff
git diff main..HEAD | oa -b codex "review this PR for security issues"

# Summarize a log file
cat /var/log/app/errors.log | oa -b claude "summarize the error patterns"
```

Piped content becomes context. Positional args become instructions. If both are provided, they're combined.

## Output

By default, `oa` prints plain text:

```text
Here's what I found...
```

With `--json`, every invocation returns a normalized response:

```json
{
  "result": "Here's what I found...",
  "session": "abc123-def456",
  "thread_id": "auth-fix",
  "backend": "claude"
}
```

With `--stream`, you get live text output with activity indicators on stderr:

```text
[activity] Read README.md
OK
```

With `--jsonl` (or `--stream --json`), output is normalized JSONL — one event per line:

```json
{"type":"session","backend":"claude","session":"abc123-def456"}
{"type":"activity","backend":"claude","session":"abc123-def456","activity":"Read README.md"}
{"type":"delta","backend":"claude","session":"abc123-def456","delta":"OK"}
{"type":"done","backend":"claude","session":"abc123-def456","result":"OK"}
```

Events are intentionally simple: `session`, optional `activity`, optional `delta`, then `done` or `error`.

## Portable threads

Threads let `oa` own the conversation history instead of relying on a single backend's session.

- **Same backend, same thread**: reuses that backend's native session when it was the last to contribute.
- **Different backend**: rebuilds context from saved turns and continues on the new backend.
- **Concurrent-safe**: thread files are locked during read/write, so a bot and a cron job can safely share a thread.
- `--thread` and `--session` are mutually exclusive.
- Threads are stored locally in `~/.local/state/oneagent/threads/<id>.json`.

Use `oa thread compact <id>` to summarize older turns and keep long-running threads manageable.

### Post-run hooks

Run a command after a thread turn completes with `--on-complete`. The result is piped to the command's stdin:

```sh
# Notify a webhook after every turn
oa -t daily -b claude --on-complete 'curl -s -X POST https://hooks.example.com/notify -d @-' \
  "summarize today's action items"
```

Environment variables `OA_THREAD_ID`, `OA_BACKEND`, `OA_SESSION`, and `OA_SOURCE` are set for the hook. Hooks are best-effort — failures are logged but don't affect the response.

## Configuration

`oa` ships with built-in defaults for `claude`, `codex`, `opencode`, and `pi` — no config file needed.

To override a built-in backend or add a new one, create `~/.config/oneagent/backends.json`:

```json
{
  "my-agent": {
    "run": "my-agent --prompt {prompt} --model {model}",
    "format": "json",
    "result": "output.text",
    "session": "session_id"
  }
}
```

Same-named entries replace the built-in. New entries are added alongside defaults. Use `-c /path/to/backends.json` to load only a specific file.

For the full config schema, field reference, match conditions, and example backends, see [docs/config.md](./docs/config.md).

## Use as a library

```sh
go get github.com/1broseidon/oneagent@latest
```

```go
import "github.com/1broseidon/oneagent"

backends, _ := oneagent.LoadBackends("")
client := oneagent.Client{Backends: backends}

// One-shot
resp := client.Run(oneagent.RunOpts{
    Backend: "claude",
    Prompt:  "explain this code",
    CWD:     "/path/to/project",
})
fmt.Println(resp.Result)

// Streaming
client.RunStream(oneagent.RunOpts{
    Backend: "claude",
    Prompt:  "review the repo",
}, func(ev oneagent.StreamEvent) {
    fmt.Print(ev.Delta)
})

// Portable threads
resp = client.RunWithThread(oneagent.RunOpts{
    Backend:  "claude",
    ThreadID: "auth-fix",
    Prompt:   "continue debugging",
})
```

For the full library API, streaming details, custom thread storage, and integration patterns, see [docs/library.md](./docs/library.md).

## Supported backends

| Backend  | CLI            | Session resume       |
|----------|----------------|----------------------|
| Claude   | `claude`       | `--resume`           |
| Codex    | `codex exec`   | `codex exec resume`  |
| OpenCode | `opencode run` | `--session`          |
| Pi       | `pi`           | `--session`          |

Any CLI that outputs JSON or line-delimited JSON can be added via [config](./docs/config.md).

## Docs

- [Changelog](./CHANGELOG.md)
- [Go library guide](./docs/library.md)
- [Backend config reference](./docs/config.md)
- [Integration example](./docs/examples/consumer.md)
- [Agent skill for dispatch](./skills/oa-dispatch/)
- [Troubleshooting](./docs/troubleshooting.md)

## License

[MIT](./LICENSE)
