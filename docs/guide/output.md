# Output Formats

`oa` normalizes output across all backends into consistent formats.

## Plain Text (default)

```sh
oa "explain this codebase"
```

```text
Here's what I found...
```

## JSON

With `--json`, every invocation returns a normalized response:

```sh
oa --json "explain this codebase"
```

```json
{
  "result": "Here's what I found...",
  "session": "abc123-def456",
  "thread_id": "auth-fix",
  "backend": "claude"
}
```

## Streaming

With `--stream`, live text output with activity indicators on stderr:

```sh
oa --stream "review the repo"
```

```text
[activity] Read README.md
OK
```

## JSONL

With `--jsonl` (or `--stream --json`), normalized events — one per line:

```sh
oa --jsonl "review the repo"
```

```json
{"type":"session","backend":"claude","session":"abc123-def456"}
{"type":"activity","backend":"claude","session":"abc123-def456","activity":"Read README.md"}
{"type":"delta","backend":"claude","session":"abc123-def456","delta":"OK"}
{"type":"done","backend":"claude","session":"abc123-def456","result":"OK"}
```

Events are intentionally simple: `session`, optional `activity`, optional `delta`, then `done` or `error`.
