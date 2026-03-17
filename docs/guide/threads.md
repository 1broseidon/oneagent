# Portable Threads

Threads let `oa` own the conversation history instead of relying on a single backend's session.

## Basic Usage

```sh
# Start a thread
oa -t auth-fix "investigate the failing auth tests"

# Continue on a different backend
oa -b codex -t auth-fix "patch the bug"

# Summarize on another
oa -b claude -t auth-fix "summarize what changed"
```

## How It Works

- **Same backend, same thread**: reuses that backend's native session when it was the last to contribute.
- **Different backend**: rebuilds context from saved turns and continues on the new backend.
- **Concurrent-safe**: thread files are locked during read/write, so multiple processes can safely share a thread.
- `--thread` and `--session` are mutually exclusive.
- Threads are stored locally in `~/.local/state/oneagent/threads/<id>.json`.

## Thread Management

```sh
oa thread list
oa thread show auth-fix
oa thread compact auth-fix
```

Use `compact` to summarize older turns and keep long-running threads manageable.

## Post-Run Hooks

Run a command after a thread turn completes with `--on-complete`. The result is piped to the command's stdin:

```sh
oa -t daily -b claude --on-complete 'curl -s -X POST https://hooks.example.com/notify -d @-' \
  "summarize today's action items"
```

Environment variables `OA_THREAD_ID`, `OA_BACKEND`, `OA_SESSION`, and `OA_SOURCE` are set for the hook. Hooks are best-effort — failures are logged but don't affect the response.
