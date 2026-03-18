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

## Mixing pipes and threads

Combine piped context with threads for powerful workflows:

```sh
# Feed test failures into a fix-iterate loop
go test ./... 2>&1 | oa -b claude -t fix "diagnose and fix these failures" && \
oa -b codex -t fix "run the tests again and verify the fix"
```

## When to use pipes vs threads

| Pattern | Use when |
|---------|----------|
| `cat file \| oa "instruction"` | You want to pass specific content as context for a one-shot task |
| `oa -t id "instruction"` | You want conversation continuity across multiple steps |
| `cmd \| oa -t id "instruction"` | Both — pass content and maintain conversation history |
| `oa -t id "step 1" && oa -t id "step 2"` | Sequential multi-agent workflow with shared context |

## Why not Unix pipes between agents?

Unix pipes (`a | b`) run both commands concurrently and stream stdout from `a` into stdin of `b`. Agent workflows are different — the second agent needs the first to finish and commit its file changes before it can review them. That's a sequential dependency, not a data stream.

Use `&&` for agent sequencing and `|` for feeding content into a single agent.
