# Backend Config Reference

Backend config files tell `oa` how to call each agent CLI, parse its output, and manage sessions. You only need a config file if you want to override a built-in default or add a new agent.

## How Config is Loaded

`oneagent` ships with embedded defaults for `claude`, `codex`, `opencode`, and `pi`.

When you call:

```go
oneagent.LoadBackends("")
```

the embedded defaults load first, then `~/.config/oneagent/backends.json` is merged on top. Same-named entries replace the built-in. New entries are added alongside.

If you pass an explicit path, only that file is loaded (no embedded defaults).

## Config Schema

Each backend is a JSON object with these fields:

| Field | Required | Description |
|-------|----------|-------------|
| `run` | yes | Command template. Variables: `{prompt}`, `{model}`, `{cwd}`, `{session}` |
| `resume` | no | Full resume command, or a patch like `+ --resume {session}` that inserts flags after `{prompt}` |
| `system` | no | System prompt prepended to the first message when there is no active session |
| `format` | yes | Output format: `json` (single object) or `jsonl` (line-delimited events) |
| `activity` | no | Dot-path or template for activity messages (e.g., "reading a file") |
| `activity_when` | no | Match condition for activity events |
| `delta` | no | Dot-path for streamed text chunks |
| `delta_when` | no | Match condition for streamed text chunks |
| `result` | yes | Dot-path for the final result text |
| `result_when` | no | Match condition for result events |
| `result_append` | no | If `true`, accumulate result text across multiple matching events |
| `session` | yes | Dot-path for the native session/thread ID |
| `session_when` | no | Match condition for session events |
| `error` | no | Dot-path for error text |
| `error_when` | no | Match condition for error events |
| `model` | no | Default model when none is passed at runtime |
| `paths` | no | Additional directories to search for the CLI binary (supports `~`). Checked when the binary isn't in `$PATH` |
| `prompt_stdin` | no | If `true`, pass the prompt via stdin instead of as a command-line argument. Keeps prompts out of `ps` output |
| `pre_run` | no | Shell command to run before every invocation of this backend. Exit non-zero aborts the run |
| `post_run` | no | Shell command to run after every invocation. Result piped to stdin. Best-effort |
| `probe` | no | Fast shell command to verify the backend is ready (e.g. `claude --version`). Used by `PreflightCheck` to catch missing API keys or auth issues before work is queued. Must exit 0 to pass |

## Template Variables

Command templates use `{variable}` placeholders:

- `{prompt}` — the user's prompt text
- `{model}` — model override (from `-m` flag or config default)
- `{cwd}` — working directory (from `-C` flag)
- `{session}` — native session ID for resume

When a variable is empty, both the variable and its preceding flag are dropped. So `--model {model}` disappears cleanly when no model is specified. Inline assignment forms like `-c model_reasoning_effort={thinking}` also disappear cleanly when the placeholder is empty.

## Match Conditions

The `*_when` fields use `key=value` syntax with dot-paths. Use `&` to combine conditions:

```text
type=item.completed
type=message_update&assistantMessageEvent.type=text_delta
type=result&is_error=true
```

Boolean and numeric values are compared as strings.

## Activity Templates

The `activity` field can be a plain dot-path or a template with `{...}` placeholders:

```json
"activity": "{message.content.0.name} {message.content.0.input.file_path}"
```

Numeric path segments index into arrays, so `.0.` is valid.

## Command Execution

`run` and `resume` are tokenized into an argument list without invoking a shell. Quotes and backslash escapes work for readability, but commands are executed directly — no shell expansion.

## Example Backends

These are the embedded defaults that ship with `oa`. All backends use `prompt_stdin` to pass prompts via stdin instead of command-line arguments, keeping prompt content out of `ps` output. Claude's `-p` mode only allows reads, so `--dangerously-skip-permissions` is required for writes and bash. Codex's `--dangerously-bypass-approvals-and-sandbox` disables the bubblewrap sandbox which fails on some kernels. Pi and OpenCode have no permission restrictions in non-interactive mode. Override any of these in `~/.config/oneagent/backends.json` if you need different flags.

**Claude:**

```json
{
  "run": "claude -p {prompt} --model {model} --output-format stream-json --include-partial-messages --verbose --dangerously-skip-permissions",
  "resume": "+ --resume {session}",
  "format": "jsonl",
  "prompt_stdin": true,
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
  "model": "sonnet",
  "paths": ["~/.claude/local", "~/.local/bin", "~/.npm-global/bin"]
}
```

**Codex:**

```json
{
  "run": "codex exec - --json --dangerously-bypass-approvals-and-sandbox -C {cwd} --skip-git-repo-check",
  "resume": "codex exec resume {session} - --json --dangerously-bypass-approvals-and-sandbox --skip-git-repo-check",
  "format": "jsonl",
  "prompt_stdin": true,
  "activity": "{item.command}",
  "activity_when": "type=item.started&item.type=command_execution",
  "delta": "assistantMessageEvent.delta",
  "delta_when": "type=message_update&assistantMessageEvent.type=text_delta",
  "result": "item.text",
  "result_when": "type=item.completed",
  "session": "thread_id",
  "session_when": "type=thread.started",
  "error": "message",
  "error_when": "type=error",
  "paths": ["~/.local/bin", "~/.npm-global/bin"]
}
```

**OpenCode:**

```json
{
  "run": "opencode run {prompt} --format json --dir {cwd} --model {model}",
  "resume": "+ --session {session}",
  "format": "jsonl",
  "prompt_stdin": true,
  "activity": "{part.tool} {part.state.input.filePath}",
  "activity_when": "type=tool_use",
  "result": "part.text",
  "result_when": "type=text",
  "session": "sessionID",
  "session_when": "type=step_start",
  "error": "part.text",
  "error_when": "type=error",
  "paths": ["~/.opencode/bin", "~/.local/bin"]
}
```

**Pi:**

```json
{
  "run": "pi -p {prompt} --mode json --model {model}",
  "resume": "+ --session {session}",
  "format": "jsonl",
  "prompt_stdin": true,
  "activity": "{assistantMessageEvent.toolCall.name} {assistantMessageEvent.toolCall.arguments.path}",
  "activity_when": "type=message_update&assistantMessageEvent.type=toolcall_end",
  "delta": "assistantMessageEvent.delta",
  "delta_when": "type=message_update&assistantMessageEvent.type=text_delta",
  "result": "message.content.0.text",
  "result_when": "type=message_end&message.role=assistant&message.content.0.type=text&message.stopReason=stop",
  "session": "id",
  "session_when": "type=session",
  "error": "message",
  "error_when": "type=error",
  "paths": ["~/.local/bin", "~/.npm-global/bin"]
}
```
