# oneagent

Config-driven multi-agent CLI.

`oa` gives Claude, Codex, OpenCode, Pi, and any future agent CLI a single normalized interface. Every backend gets the same flags, the same JSON and streaming output, and portable conversation threads. Adding a new agent is a JSON config edit — no code required.

## Prerequisites

- [Go 1.21+](https://go.dev/dl/) (for install via `go install`)
- At least one supported agent CLI installed and signed in (e.g., `claude`, `codex`, `opencode`, or `pi`)

## Install

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

# Thread management
oa thread list
oa thread show auth-fix
oa thread compact auth-fix
```

Works out of the box if `claude`, `codex`, `opencode`, or `pi` is installed and signed in.

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
- `--thread` and `--session` are mutually exclusive.
- Threads are stored locally in `~/.local/state/oneagent/threads/<id>.json`.

Use `oa thread compact <id>` to summarize older turns and keep long-running threads manageable.

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

## License

[MIT](./LICENSE)
