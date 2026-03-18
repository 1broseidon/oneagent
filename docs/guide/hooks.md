# Hooks

Hooks run commands before or after agent execution. Use them for setup, teardown, notifications, logging, or any side effect that should happen around an agent call.

## CLI hooks

```sh
# Pre-run: set up environment before the agent runs
oa -b codex --pre-run 'echo "Starting $OA_BACKEND..."' "fix the bug"

# Post-run: notify when done, result piped to stdin
oa -b claude --post-run 'notify-send "oa" "Done: $(head -c 100)"' "review the repo"

# Both together
oa -b codex -t feat \
  --pre-run 'echo "Starting on branch $(git branch --show-current)"' \
  --post-run 'notify-send "oa" "$OA_BACKEND finished ($OA_EXIT)"' \
  "add input validation"
```

Pre-run hooks abort the run if they exit non-zero. Post-run hooks are best-effort — failures are logged but don't affect the response.

## Config hooks

Set hooks per-backend in `~/.config/oneagent/backends.json` so they run on every invocation:

```json
{
  "codex": {
    "pre_run": "echo \"[$(date)] $OA_BACKEND starting\" >> ~/.oa-audit.log",
    "post_run": "echo \"[$(date)] $OA_BACKEND done (exit=$OA_EXIT)\" >> ~/.oa-audit.log"
  }
}
```

Config hooks and CLI hooks stack — config runs first, then CLI. They don't override each other.

## Library hooks

Go callbacks for applications that embed oneagent:

```go
resp := client.Run(oneagent.RunOpts{
    Backend: "claude",
    Prompt:  "fix the failing tests",
    PreRun: func(opts *oneagent.RunOpts) error {
        opts.CWD = createWorktree(opts.ThreadID)
        return nil // return error to abort
    },
    PostRun: func(ctx *oneagent.HookContext) {
        log.Printf("backend=%s result_len=%d",
            ctx.Response.Backend, len(ctx.Response.Result))
    },
})
```

`PreRun` receives a mutable `*RunOpts` — it can change the CWD, prompt, model, or anything else before execution. Return an error to abort. `PostRun` receives a `HookContext` with both the original opts and the response.

## Environment variables

Hooks run via `sh -c` and receive these environment variables:

**Pre-run:**

| Variable | Value |
|----------|-------|
| `OA_BACKEND` | Backend name |
| `OA_THREAD_ID` | Thread ID (if set) |
| `OA_SOURCE` | Source attribution (if set) |
| `OA_MODEL` | Model being used |
| `OA_CWD` | Working directory |

**Post-run** (all of above, plus):

| Variable | Value |
|----------|-------|
| `OA_SESSION` | Native session ID |
| `OA_ERROR` | Error message (empty on success) |
| `OA_EXIT` | `0` for success, `1` for error |

Post-run hooks also receive the full result text on stdin.

## Execution order

When hooks are configured at multiple layers, they execute in this order:

1. Library `PreRun` callback
2. Config `pre_run` shell command
3. CLI `--pre-run` shell command
4. **Agent executes**
5. Config `post_run` shell command
6. CLI `--post-run` shell command
7. Library `PostRun` callback

Any pre-run hook can abort the run by exiting non-zero (shell) or returning an error (Go). Post-run hooks are always best-effort.

## Recipes

### Desktop notifications

```sh
oa -b codex \
  --pre-run 'notify-send "oa" "Starting $OA_BACKEND..."' \
  --post-run 'notify-send "oa" "Done: $(head -c 200)"' \
  "fix the auth bug"
```

### Audit logging

```json
{
  "claude": {
    "pre_run": "echo \"[$(date -Iseconds)] START backend=$OA_BACKEND thread=$OA_THREAD_ID\" >> ~/.oa-audit.log",
    "post_run": "echo \"[$(date -Iseconds)] END backend=$OA_BACKEND exit=$OA_EXIT\" >> ~/.oa-audit.log"
  }
}
```

### Worktree isolation

```sh
oa -b codex -t refactor \
  --pre-run 'git worktree add -b oa-$OA_THREAD_ID ../oa-$OA_THREAD_ID HEAD' \
  --post-run 'echo "Review: cd ../oa-$OA_THREAD_ID"' \
  -C "../oa-$OA_THREAD_ID" \
  "extract the auth middleware into its own package"
```

### Slack notification

```sh
oa -b claude -t daily \
  --post-run 'curl -s -X POST "$SLACK_WEBHOOK" -H "Content-Type: application/json" -d "{\"text\": \"$(head -c 3000)\"}"' \
  "summarize today's git activity and open PRs"
```

### Quality gate chain

```sh
oa -b codex -t feat "implement the feature" && \
go test ./... && \
oa -b claude -t feat "security review, fix any issues" && \
go test ./... && \
git diff main | oa -b claude "write a PR description"
```

Note: quality gates (test runs) are chain steps, not hooks. Hooks are for side effects. Gates are for control flow.
