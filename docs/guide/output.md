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
{"type":"start","run_id":"run-...","ts":"2026-03-22T15:04:05Z","backend":"claude"}
{"type":"session","run_id":"run-...","ts":"2026-03-22T15:04:05Z","backend":"claude","session":"abc123-def456"}
{"type":"activity","run_id":"run-...","ts":"2026-03-22T15:04:06Z","backend":"claude","session":"abc123-def456","activity":"Read README.md"}
{"type":"heartbeat","run_id":"run-...","ts":"2026-03-22T15:04:15Z","backend":"claude"}
{"type":"delta","run_id":"run-...","ts":"2026-03-22T15:04:16Z","backend":"claude","session":"abc123-def456","delta":"OK"}
{"type":"done","run_id":"run-...","ts":"2026-03-22T15:04:16Z","backend":"claude","session":"abc123-def456","result":"OK"}
```

Events are intentionally simple: `start`, optional `session` / `activity` / `delta`, library-emitted `heartbeat` while the process is alive, then `done` or `error`. `run_id` and `ts` let supervisors distinguish attempts and detect stalls without backend-specific parsing.
