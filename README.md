# oneagent

One interface for every AI coding agent. Config-driven, zero code changes to add new backends.

`oa` wraps any agent CLI (Claude, Codex, Cursor, OpenCode, Cline, Pi, ...) behind one interface. The CLI defaults to human-friendly text output, with `--json` for final machine-readable output and `--stream --json` for normalized JSONL events. Define backends in a config file with command templates, output parsing rules, and session support. Add new agents by editing a JSON file — no code required.

## Install

```sh
go install github.com/1broseidon/oneagent/cmd/oa@latest
```

## Quick start

```sh
# Talk to Claude
oa "explain this codebase"

# Machine-readable final JSON
oa --json "explain this codebase"

# Use a different backend
oa -b codex "fix the auth bug"

# Live text stream
oa --stream "review the repo"

# Normalized JSONL stream
oa --stream --json "review the repo"

# Start or continue a portable thread
oa -t auth-fix "investigate the failing auth tests"
oa -b codex -t auth-fix "patch the bug"
oa -b claude -t auth-fix "summarize what changed"

# Specify model and working directory
oa -b pi -m "google/gemini-2.5-pro" -C ~/project "add tests"

# Resume a session
oa -b claude -s abc123 "now refactor it"

# Inspect or compact saved threads
oa thread list
oa thread show auth-fix
oa thread compact auth-fix

# List configured backends
oa list
```

Built-in defaults ship for `claude`, `codex`, `opencode`, and `pi`, so `oa` works on first run if those CLIs are installed and signed in.

## Output

By default, `oa` prints plain text:

```text
Here's what I found...
```

With `--json`, every invocation returns normalized JSON:

```json
{
  "result": "Here's what I found...",
  "session": "abc123-def456",
  "thread_id": "auth-fix",
  "backend": "claude"
}
```

On error with `--json`:

```json
{
  "result": "",
  "session": "",
  "thread_id": "auth-fix",
  "backend": "cline",
  "error": "Unauthorized: Please sign in to Cline before trying again."
}
```

With `--stream`, `oa` prints a human-friendly live stream:

```text
[activity] Read README.md
OK
```

With `--stream --json`, output is normalized JSONL:

```json
{"type":"session","backend":"claude","session":"abc123-def456"}
{"type":"activity","backend":"claude","session":"abc123-def456","activity":"Read README.md"}
{"type":"delta","backend":"claude","session":"abc123-def456","delta":"OK"}
{"type":"done","backend":"claude","session":"abc123-def456","result":"OK"}
```

The normalized stream shape is intentionally small: `session`, optional `activity`, optional `delta`, then final `done` or `error`.

## Portable threads

Use `-t/--thread` to make `oneagent` own the conversation history instead of relying only on a backend-native session ID.

Thread behavior:

- Same backend, same thread: reuse that backend's native session when that backend was the last contributor.
- Switched backends: rebuild context from the saved thread summary and turns, then continue on the new backend.
- `--thread` and `--session` are mutually exclusive.
- Threads are stored locally in `~/.local/state/oneagent/threads/<id>.json`.

Thread commands:

```sh
oa thread list
oa thread show <id>
oa thread compact <id> [-b backend]
```

`oa thread compact` summarizes older turns and keeps recent turns verbatim so long-running threads stay portable without growing without bound.

## Configuration

`oa` embeds default configs for `claude`, `codex`, `opencode`, and `pi`.

If `~/.config/oneagent/backends.json` exists, any same-named backend replaces the built-in definition and any new backend is added alongside the defaults.

Use `-c /path/to/backends.json` to load only a specific config file.

Example override file:

```json
{
  "claude": {
    "run": "claude -p {prompt} --model {model} --output-format stream-json --include-partial-messages --verbose",
    "resume": "+ --resume {session}",
    "system": "You are a helpful assistant.",
    "format": "jsonl",
    "activity": "{message.content.0.name} {message.content.0.input.file_path}",
    "activity_when": "type=assistant&message.content.0.type=tool_use",
    "delta": "event.delta.text",
    "delta_when": "type=stream_event&event.type=content_block_delta&event.delta.type=text_delta",
    "result": "result",
    "result_when": "type=result&is_error=false",
    "session": "session_id",
    "session_when": "type=system",
    "error": "result",
    "error_when": "type=result&is_error=true",
    "model": "sonnet"
  },
  "codex": {
    "run": "codex exec {prompt} --json --full-auto -C {cwd}",
    "resume": "codex exec resume {session} {prompt} --json --full-auto",
    "format": "jsonl",
    "activity": "{item.command}",
    "activity_when": "type=item.started&item.type=command_execution",
    "delta": "assistantMessageEvent.delta",
    "delta_when": "type=message_update&assistantMessageEvent.type=text_delta",
    "result": "item.text",
    "result_when": "type=item.completed",
    "session": "thread_id",
    "session_when": "type=thread.started",
    "error": "message",
    "error_when": "type=error"
  }
}
```

### Backend fields

| Field | Required | Description |
|-------|----------|-------------|
| `run` | yes | Command template as a string. Variables: `{prompt}`, `{model}`, `{cwd}`, `{session}` |
| `resume` | no | Resume command as a full string, or a patch like `+ --resume {session}` inserted after `{prompt}` |
| `system` | no | Prepended to prompt on first message (no active session) |
| `format` | yes | Output format: `json` (single object) or `jsonl` (line-delimited events) |
| `activity` | no | Dot-path or template for a generic pre-final activity message |
| `activity_when` | no | Match condition for activity events |
| `delta` | no | Dot-path to a streaming text chunk for `--stream` |
| `delta_when` | no | Match condition for streaming delta events |
| `result` | yes | Dot-path to result text (e.g. `result`, `item.text`, `assistantMessageEvent.delta`) |
| `result_when` | no | Match condition for jsonl (e.g. `type=item.completed`). Supports `&` for AND |
| `result_append` | no | If `true`, accumulate result across multiple matching events (for streaming deltas) |
| `session` | yes | Dot-path to session/thread ID |
| `session_when` | no | Match condition for jsonl session events |
| `error` | no | Dot-path to error message |
| `error_when` | no | Match condition for error events |
| `model` | no | Fallback model when none specified |

### Template variables

When a variable resolves to empty, both the variable and its preceding flag are dropped. So `--model {model}` cleanly disappears when no model is set, letting the backend use its own default.

### Command strings

`run` and `resume` are tokenized into argv without invoking a shell. Quotes and backslash escapes are supported for readability, but commands are still executed directly.

### Match conditions

`activity_when`, `delta_when`, `result_when`, `session_when`, and `error_when` use `key=value` syntax with dot-paths:

```
type=item.completed                     # single condition
type=message_update&assistantMessageEvent.type=text_delta   # AND conditions
```

Activity templates can interpolate dot-paths with `{...}` placeholders. Numeric path segments index arrays, so `{message.content.0.name}` is valid.

## Use as a library

```go
import "github.com/1broseidon/oneagent"

backends, _ := oneagent.LoadBackends("") // embedded defaults + ~/.config overlay

resp := oneagent.Run(backends, oneagent.RunOpts{
    Backend: "claude",
    Prompt:  "explain this code",
    CWD:     "/path/to/project",
})

fmt.Println(resp.Result)
fmt.Println(resp.Session) // for resume
```

Streaming from Go:

```go
oneagent.RunStream(backends, oneagent.RunOpts{
    Backend: "claude",
    Prompt:  "review the repo",
    CWD:     "/path/to/project",
}, func(event oneagent.StreamEvent) {
    fmt.Println(event.Type, event.Activity, event.Delta, event.Result)
})
```

Portable threads from Go:

```go
resp := oneagent.RunWithThread(backends, oneagent.RunOpts{
    Backend:  "claude",
    ThreadID: "auth-fix",
    Prompt:   "continue debugging",
    CWD:      "/path/to/project",
})

fmt.Println(resp.Result)
fmt.Println(resp.ThreadID)
```

## Supported backends

Built-in defaults:

| Backend | CLI | Format | Session resume |
|---------|-----|--------|---------------|
| Claude | `claude` | jsonl | `--resume` |
| Codex | `codex exec` | jsonl | `codex exec resume` |
| OpenCode | `opencode run` | jsonl | `--session` |
| Pi | `pi` | jsonl | `--session` |

Any CLI that outputs JSON or line-delimited JSON can be added via config.

## License

MIT
