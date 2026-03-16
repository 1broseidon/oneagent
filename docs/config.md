# Backend Config

Backends are defined in JSON and compiled into canonical `Backend` structs at runtime.

## Loading Config

`oneagent` ships embedded defaults for:

- `claude`
- `codex`
- `opencode`
- `pi`

When you call:

```go
oneagent.LoadBackends("")
```

the embedded defaults are loaded first, then `~/.config/oneagent/backends.json` is merged on top.

If you pass an explicit path, only that file is loaded.

## Schema

Each backend entry can define:

| Field | Required | Description |
|-------|----------|-------------|
| `run` | yes | Command template. Variables: `{prompt}`, `{model}`, `{cwd}`, `{session}` |
| `resume` | no | Full resume command or a patch like `+ --resume {session}` |
| `system` | no | Prepended to the first prompt when there is no active native session |
| `format` | yes | `json` or `jsonl` |
| `activity` | no | Dot-path or template for a generic pre-final activity message |
| `activity_when` | no | Match condition for activity events |
| `delta` | no | Dot-path for streamed text chunks |
| `delta_when` | no | Match condition for streamed text chunks |
| `result` | yes | Dot-path for the canonical final result |
| `result_when` | no | Match condition for final result events |
| `result_append` | no | Accumulate result across multiple matches |
| `session` | yes | Dot-path for native session/thread ID |
| `session_when` | no | Match condition for session events |
| `error` | no | Dot-path for error text |
| `error_when` | no | Match condition for error events |
| `model` | no | Default model when none is passed at runtime |

## Match Conditions

Conditions use `key=value` with dot-paths and `&` for AND:

```text
type=item.completed
type=message_update&assistantMessageEvent.type=text_delta
```

Boolean values are matched using their string form:

```text
type=result&is_error=true
```

## Templates

`activity` can be either:

- a plain dot-path
- or a template with `{...}` placeholders

Example:

```json
"activity": "{message.content.0.name} {message.content.0.input.file_path}"
```

Array indexes are supported in paths, so `.0.` is valid.

## Example Backends

Claude:

```json
{
  "run": "claude -p {prompt} --model {model} --output-format stream-json --include-partial-messages --verbose",
  "resume": "+ --resume {session}",
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
}
```

Codex:

```json
{
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
```

Pi:

```json
{
  "run": "pi -p {prompt} --mode json --model {model}",
  "resume": "+ --session {session}",
  "format": "jsonl",
  "activity": "{assistantMessageEvent.toolCall.name} {assistantMessageEvent.toolCall.arguments.path}",
  "activity_when": "type=message_update&assistantMessageEvent.type=toolcall_end",
  "result": "assistantMessageEvent.delta",
  "result_when": "type=message_update&assistantMessageEvent.type=text_delta",
  "result_append": true,
  "session": "id",
  "session_when": "type=session",
  "error": "message",
  "error_when": "type=error"
}
```

## Runtime Behavior

When a template variable resolves to empty, both the value and its preceding flag are dropped. That lets commands like:

```text
--model {model}
```

disappear cleanly when no model override is provided.

Command strings are tokenized into argv without invoking a shell.
