# Pipes and Pipelines

## Piping content into oa

Pipe any content into `oa` as context. If you also provide positional arguments, those become the instructions:

```sh
# Pipe as the full prompt
echo "what is 2+2?" | oa -b claude

# Pipe as context, args as instructions
cat internal/auth/handler.go | oa -b codex "review this for security issues"

# Git diff as context
git diff | oa -b claude "explain what changed and why"

# Test output as context
go test ./... 2>&1 | oa -b claude "diagnose these failures"

# Log analysis
cat /var/log/app/errors.log | oa -b claude "summarize the error patterns"
```

When both pipe and args are present, they're combined: the piped content comes first as context, followed by the positional args as instructions.

## Sequential pipelines

Chain agents with `&&` and shared threads for multi-step workflows. Each step sees the file changes and conversation context from previous steps:

```sh
# Build → review → document
oa -b codex -t feat "add input validation to the signup handler" && \
oa -b codex -t feat "review the changes, run tests, report any issues" && \
git diff | oa -b claude "write a changelog entry for this change"
```

This works because:

- `&&` runs the next command only if the previous one succeeds
- `-t feat` shares a thread across all steps — each agent sees the full conversation history
- The thread carries context even when switching backends
- `git diff` pipes the actual file changes into the final step

## Self-healing loop

Run tests, pipe failures to an agent, repeat until green:

```sh
for i in 1 2 3; do
  go test ./... 2>&1 && break
  go test ./... 2>&1 | oa -b codex -t fix "fix these test failures, attempt $i of 3"
done
```

## Multi-agent review

Different agents with different perspectives, results merged:

```sh
git diff main | oa -b claude "security review" > /tmp/security.md && \
git diff main | oa -b codex "performance review" > /tmp/perf.md && \
cat /tmp/security.md /tmp/perf.md | oa -b claude "synthesize into one summary with action items"
```

## Daily digest with pipes + hooks + threads

Gather context from multiple sources, synthesize, notify:

```sh
(
  echo "=== Git activity ==="
  git log --oneline --since="yesterday"
  echo "=== Open PRs ==="
  gh pr list --state open
  echo "=== Failing checks ==="
  gh run list --status failure --limit 5
) | oa -b claude -t "daily-$(date +%Y%m%d)" \
  --post-run 'curl -s -X POST "$SLACK_WEBHOOK" -H "Content-Type: application/json" -d "{\"text\": \"$(head -c 3000)\"}"' \
  "daily standup summary: what happened yesterday, what needs attention today"
```

Pipe gathers context. Agent synthesizes. Post-run hook pushes to Slack. Thread preserves history so tomorrow's run has context from today.

## The four primitives

`oa` has four composable primitives. Each is useful on its own; together they're an agent orchestration layer in plain shell.

| Primitive | What it does | Example |
|-----------|-------------|---------|
| **Pipes** | Feed content into an agent as context | `git diff \| oa "review"` |
| **Chains** | Sequential multi-agent workflows | `oa "build" && oa "test"` |
| **Hooks** | Lifecycle automation around each call | `--post-run 'notify-send ...'` |
| **Threads** | Conversation memory across steps | `-t feat` shared across agents |

No SDK, no framework, no orchestration server. Standard shell.

## When to use what

| Pattern | Use when |
|---------|----------|
| `cat file \| oa "instruction"` | Pass specific content as context |
| `oa -t id "step 1" && oa -t id "step 2"` | Sequential workflow with shared memory |
| `--pre-run` / `--post-run` | Setup, teardown, notifications, logging |
| `-t id` | Conversation continuity across steps or sessions |
| All four combined | Complex workflows: isolated worktree, multi-agent pipeline, notify on completion |

See [Hooks](/guide/hooks) for the full hook reference and more recipes.

## Why not Unix pipes between agents?

Unix pipes (`a | b`) run both commands concurrently and stream stdout from `a` into stdin of `b`. Agent workflows are different — the second agent needs the first to finish and commit its file changes before it can review them. That's a sequential dependency, not a data stream.

Use `&&` for agent sequencing and `|` for feeding content into a single agent.
