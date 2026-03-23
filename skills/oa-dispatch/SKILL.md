---
name: oa-dispatch
description: Dispatch coding tasks to other AI agents via oneagent (oa). Use when you want to delegate file edits, code generation, reviews, or other scoped tasks to a different model or backend — for example, sending a targeted edit to Codex while continuing your own work.
compatibility: Requires oa CLI (go install github.com/1broseidon/oneagent/cmd/oa@latest) and at least one agent backend (claude, codex, opencode, or pi) installed and signed in.
---

# Dispatch Tasks to Other Agents

Use `oa` to delegate scoped coding tasks to a different AI model or backend. This is useful when you want to parallelize work, use a specific model's strengths, or offload a well-defined edit while you continue working.

## When to dispatch

Dispatch works best for **targeted, self-contained tasks** with clear instructions:

- Edit a specific file or function
- Generate a test suite for a module
- Refactor a block of code to match a pattern
- Review a file for a specific concern (security, performance, etc.)

Do not dispatch vague or open-ended tasks. The receiving agent has no access to your conversation context — the prompt must be self-contained.

## Basic command

Always pipe the prompt into `oa` instead of passing it as an argument. This keeps prompt content out of process listings (`ps aux`):

```bash
echo "<prompt>" | oa -b <backend> -m <model> --thinking <level>
```

All flags are optional. `--thinking` controls reasoning effort — supported levels vary by backend (e.g. `low`, `medium`, `high`). Omit it to use the backend's default.

For background execution with streaming visibility:

```bash
echo "Edit path/to/file.go: <specific instructions>" | oa -b claude --jsonl
```

For multi-line prompts, use a heredoc:

```bash
cat <<'EOF' | oa -b codex --jsonl
Edit internal/auth/handler.go: add rate limiting to the Login function.
Use x/time/rate with a limit of 5 requests per second per IP.
Do not modify any other functions. Do not ask for confirmation, just make the edits.
Run go test ./internal/auth/... when done.
EOF
```

## Constructing effective prompts

The prompt must include everything the agent needs. Always include:

1. **The file path** to edit or read
2. **What to do** — be specific about the change
3. **Constraints** — what not to change, patterns to follow, tests to pass

Good prompt:
```
Edit internal/auth/handler.go: Add rate limiting to the Login function.
Use x/time/rate with a limit of 5 requests per second per IP.
Do not modify any other functions. Do not ask for confirmation, just make the edits.
Run go test ./internal/auth/... when done.
```

Bad prompt:
```
Add rate limiting to the auth system
```

Always include **"Do not ask for confirmation, just make the edits"** or similar. Without this, some backends may plan or brainstorm instead of making changes, especially if they have skills or settings that encourage a "plan first" workflow.

## Available backends and models

Built-in backends:

| Backend    | Command          | `--thinking` | Best for                    |
|------------|------------------|--------------|-----------------------------|
| `claude`   | `oa -b claude`   | yes          | Default, general-purpose    |
| `codex`    | `oa -b codex`    | yes          | Sandboxed execution         |
| `opencode` | `oa -b opencode` | no           | OpenCode-supported models   |
| `pi`       | `oa -b pi`       | yes          | Wide model selection via Pi |
| `gemini`   | `oa -b gemini`   | no           | Gemini CLI models           |

To use a specific model or thinking level:

```bash
echo "prompt" | oa -b pi -m openai-codex/gpt-5.4
echo "prompt" | oa -b claude -m sonnet --thinking high
echo "prompt" | oa -b codex --thinking high
```

Run `oa list` to see all configured backends.

## Running in the background

Run the dispatch as a background task so you can continue working:

```bash
# Background with JSONL streaming (can inspect progress)
echo "Edit file.go: add error handling to ProcessOrder" | oa -b codex --jsonl &

# Or with a specific model via Pi
echo "Edit file.go: add error handling to ProcessOrder" | oa -b pi -m openai-codex/gpt-5.4 --jsonl &
```

After the task completes, verify the changes:

```bash
git diff path/to/file.go
```

**Timeouts:** Dispatched tasks can take anywhere from 30 seconds to 30 minutes depending on complexity. When running `oa` from a shell tool, set a timeout of at least 10 minutes (600000ms). The default 2-minute timeout will kill most non-trivial tasks prematurely.

**Parallelism rule:** You can dispatch multiple tasks in parallel if they edit different files. If multiple tasks touch the same file, run them sequentially — otherwise the later task may overwrite or conflict with the earlier one.

## Using threads for multi-step tasks

When a task needs follow-up work, use threads to carry context:

```bash
# First pass with Claude
echo "Refactor internal/auth/handler.go to extract middleware" | oa -b claude -t refactor-auth

# Follow-up on the same thread, different backend
echo "Now add tests for the extracted middleware" | oa -b codex -t refactor-auth
```

Threads are portable — switch backends mid-thread and context carries over automatically.

## Output modes

| Flag       | Output                              | Use when                        |
|------------|-------------------------------------|---------------------------------|
| (default)  | Plain text result                   | Quick one-shot tasks            |
| `--json`   | Normalized JSON with result/session | Parsing the final result        |
| `--jsonl`  | Streaming JSONL events              | Monitoring progress in real time|

## Verifying results

Always verify dispatched work before proceeding:

1. **Check the diff**: `git diff` to see what changed
2. **Run tests**: `go test ./...` or equivalent
3. **Read the output**: Check for errors or unexpected changes

If the result is wrong, either fix it yourself or dispatch a follow-up with more specific instructions.

## Multi-agent scripts

For complex dispatch patterns, use the bundled scripts:

- **`scripts/converse.sh`** — Two agents discuss a problem on a shared thread, alternating turns, then synthesize findings into an action plan.
  ```bash
  bash scripts/converse.sh -a claude -b codex -t 3 "the service crashes after 5 minutes under load"
  ```

- **`scripts/multi-review.sh`** — Dispatch parallel code reviews to different agents (security + performance), then merge into one summary.
  ```bash
  bash scripts/multi-review.sh
  ```

Both scripts use threads, pipes, and hooks — the same primitives available for any custom workflow.

## Gotchas

- The dispatched agent runs in your working directory but has no access to your conversation history. Every prompt must be self-contained.
- If a backend CLI is not installed or not signed in, `oa` will return an error. Check with `oa list` first.
- `--thread` and `--session` are mutually exclusive. Use threads for cross-backend continuity, sessions for single-backend resume.
- Large prompts with full file contents may hit token limits on some models. Point the agent at the file path instead of pasting contents.
