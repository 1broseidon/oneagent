---
name: dispatch
description: Delegate focused work to other configured backends via oa.
---

# Dispatch Work to Another Backend

Use `oa` to hand a well-scoped task to another configured backend while you keep working locally. This is useful when the task is concrete, self-contained, and can be validated independently.

## When to use this skill

Dispatch works best for bounded tasks such as:

- Editing one file or a small set of files
- Writing a targeted test suite
- Reviewing code for one concern like security or performance
- Running a mechanical refactor with clear constraints

Do not dispatch vague work. The receiving agent does not inherit your conversation state unless you explicitly provide the context.

## Preferred invocation pattern

Pipe the prompt into `oa` instead of passing it on the command line so the task text does not show up in process listings:

```bash
cat <<'EOF' | oa -b codex --jsonl
Edit internal/auth/handler.go to add request rate limiting.
Use x/time/rate with a limit of 5 requests per second per IP.
Do not modify any other files.
Run go test ./internal/auth/... before you finish.
EOF
```

Use `--jsonl` or `--stream --json` when you want normalized progress events. Use the default text mode for quick one-shot tasks.

## How to write the prompt

Every dispatch prompt should include:

1. The files or directories the agent should work in
2. The exact change to make
3. Constraints, patterns, or tests that must pass
4. A direct instruction to make the edits without waiting for confirmation

Good prompt:

```text
Edit cmd/server/main.go to add a --graceful-timeout flag.
Follow the existing flag parsing pattern.
Do not change any other behavior.
Run go test ./cmd/server/... before you finish.
Do not ask for confirmation; make the edits directly.
```

Bad prompt:

```text
Improve the server startup flow.
```

## Choosing a backend

Pick the backend based on the job:

- `oa -b claude` for general code generation and review
- `oa -b codex` for sandboxed editing and command execution
- `oa -b opencode` for OpenCode-managed models
- `oa -b pi -m <model>` when you want explicit model routing

Run `oa list` to see what is configured and whether a backend is installed.

## Using threads for follow-up work

When a dispatched task may need multiple turns, use a thread ID so you can continue the work later or switch backends:

```bash
echo "Extract the retry policy into a helper and update the call sites." | oa -b claude -t retry-refactor
echo "Now add tests for the helper." | oa -b codex -t retry-refactor
```

Use `--thread` for cross-backend continuity. Use `--session` only when resuming the same backend's native session.

## Verification

Always verify the result before relying on it:

1. Read the diff
2. Run the relevant tests
3. Confirm the output did not hide an error behind a partial success message

If the first dispatch misses the target, send a narrower follow-up prompt with the exact correction needed.

## Passing skills to dispatched agents

Run `oa skills list` to see available skills. If a dispatched task would benefit from a skill, pass it with `--skills`:

```bash
# Inject the catalog so the agent can discover and load skills on demand
cat <<'EOF' | oa -b codex --skills --jsonl
Refactor the auth module and add tests.
EOF

# Inject specific skill bodies inline — the agent gets the instructions immediately
cat <<'EOF' | oa -b claude --skills dispatch --jsonl
Use the dispatch skill to delegate the test writing to codex.
EOF
```

- `--skills` (bare) injects a lightweight catalog of skill names and descriptions
- `--skills name1,name2` injects full skill bodies inline — use when you know what the agent needs

Without `--skills`, no skill content is injected. This keeps simple tasks token-efficient.

## Constraints to keep in mind

- The dispatched agent runs in the working directory you provide with `-C` or the current directory if omitted.
- Prompts should be self-contained; do not assume the receiving agent sees this conversation.
- Parallel dispatch is fine when tasks touch different files. If tasks overlap on the same files, run them sequentially.
