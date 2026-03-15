# oneagent

One interface for every AI coding agent. Config-driven, zero code changes to add new backends.

`oa` wraps any agent CLI (Claude, Codex, Cursor, OpenCode, Cline, Pi, ...) behind a unified JSON interface. Define backends in a config file with command templates, output parsing rules, and session support. Add new agents by editing a JSON file — no code required.

## Install

```sh
go install github.com/1broseidon/oneagent/cmd/oa@latest
```

## Quick start

```sh
# Talk to Claude
oa "explain this codebase"

# Use a different backend
oa -b codex "fix the auth bug"

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

## Output

Every invocation returns normalized JSON:

```json
{
  "result": "Here's what I found...",
  "session": "abc123-def456",
  "thread_id": "auth-fix",
  "backend": "claude"
}
```

On error:

```json
{
  "result": "",
  "session": "",
  "thread_id": "auth-fix",
  "backend": "cline",
  "error": "Unauthorized: Please sign in to Cline before trying again."
}
```

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

Create `~/.config/oneagent/backends.json`:

```json
{
  "claude": {
    "cmd": ["claude", "-p", "{prompt}", "--model", "{model}", "--output-format", "json"],
    "resume_cmd": ["claude", "-p", "{prompt}", "--resume", "{session}", "--model", "{model}", "--output-format", "json"],
    "system_prompt": "You are a helpful assistant.",
    "format": "json",
    "result": "result",
    "session": "session_id",
    "error": "result",
    "error_when": "is_error=true",
    "default_model": "sonnet"
  },
  "codex": {
    "cmd": ["codex", "exec", "{prompt}", "--json", "--full-auto", "-C", "{cwd}"],
    "resume_cmd": ["codex", "exec", "resume", "{session}", "{prompt}", "--json", "--full-auto"],
    "format": "jsonl",
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
| `cmd` | yes | Command template as array. Variables: `{prompt}`, `{model}`, `{cwd}`, `{session}` |
| `resume_cmd` | no | Alternate template used when resuming a session |
| `system_prompt` | no | Prepended to prompt on first message (no active session) |
| `format` | yes | Output format: `json` (single object) or `jsonl` (line-delimited events) |
| `result` | yes | Dot-path to result text (e.g. `result`, `item.text`, `assistantMessageEvent.delta`) |
| `result_when` | no | Match condition for jsonl (e.g. `type=item.completed`). Supports `&` for AND |
| `result_append` | no | If `true`, accumulate result across multiple matching events (for streaming deltas) |
| `session` | yes | Dot-path to session/thread ID |
| `session_when` | no | Match condition for jsonl session events |
| `error` | no | Dot-path to error message |
| `error_when` | no | Match condition for error events |
| `default_model` | no | Fallback model when none specified |

### Template variables

When a variable resolves to empty, both the variable and its preceding flag are dropped. So `--model {model}` cleanly disappears when no model is set, letting the backend use its own default.

### Match conditions

`result_when`, `session_when`, and `error_when` use `key=value` syntax with dot-paths:

```
type=item.completed                     # single condition
type=message_update&assistantMessageEvent.type=text_delta   # AND conditions
```

## Use as a library

```go
import "github.com/1broseidon/oneagent"

backends, _ := oneagent.LoadBackends("backends.json")

resp := oneagent.Run(backends, oneagent.RunOpts{
    Backend: "claude",
    Prompt:  "explain this code",
    CWD:     "/path/to/project",
})

fmt.Println(resp.Result)
fmt.Println(resp.Session) // for resume
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

Tested and working out of the box:

| Backend | CLI | Format | Session resume |
|---------|-----|--------|---------------|
| Claude | `claude` | json | `--resume` |
| Codex | `codex exec` | jsonl | `codex exec resume` |
| OpenCode | `opencode run` | jsonl | `--session` |
| Cursor | `cursor-agent` | jsonl | `--resume` |
| Cline | `cline` | jsonl | `-T` |
| Pi | `pi` | jsonl | `--session` |

Any CLI that outputs JSON or line-delimited JSON can be added via config.

## License

MIT
