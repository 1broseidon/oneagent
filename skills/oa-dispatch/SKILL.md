---
name: oa-dispatch
description: Dispatch coding tasks to other AI agents via oneagent (oa). Use when you want to delegate file edits, code generation, reviews, or other scoped tasks to a different model or backend — for example, sending a targeted edit to gpt-5.4 via Pi while continuing your own work.
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

```bash
oa -b <backend> -m <model> "<prompt>"
```

For background execution with streaming visibility:

```bash
oa -b pi -m openai-codex/gpt-5.4 "Edit path/to/file.go: <specific instructions>" --jsonl
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
Do not modify any other functions. Run go test ./internal/auth/... when done.
```

Bad prompt:
```
Add rate limiting to the auth system
```

## Available backends and models

Built-in backends:

| Backend    | Command        | Best for                          |
|------------|----------------|-----------------------------------|
| `claude`   | `oa -b claude` | Default, general-purpose          |
| `codex`    | `oa -b codex`  | Sandboxed execution               |
| `opencode` | `oa -b opencode` | OpenCode-supported models       |
| `pi`       | `oa -b pi`     | Wide model selection via Pi       |

To use a specific model through a backend that supports model routing:

```bash
oa -b pi -m openai-codex/gpt-5.4 "prompt"
oa -b pi -m ollama/llama3.1 "prompt"
oa -b pi -m anthropic/claude-sonnet-4-6 "prompt"
```

Run `oa list` to see all configured backends.

## Running in the background

Run the dispatch as a background task so you can continue working:

```bash
# Background with JSONL streaming (can inspect progress)
oa -b pi -m openai-codex/gpt-5.4 "Edit file.go: add error handling to ProcessOrder" --jsonl &

# Or use your agent platform's background task support
```

After the task completes, verify the changes:

```bash
git diff path/to/file.go
```

## Using threads for multi-step tasks

When a task needs follow-up work, use threads to carry context:

```bash
# First pass
oa -b pi -m openai-codex/gpt-5.4 -t refactor-auth "Refactor internal/auth/handler.go to extract middleware"

# Follow-up on the same thread
oa -b pi -m openai-codex/gpt-5.4 -t refactor-auth "Now add tests for the extracted middleware"
```

Threads are portable — you can switch backends mid-thread if needed.

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

## Gotchas

- The dispatched agent runs in your working directory but has no access to your conversation history. Every prompt must be self-contained.
- If a backend CLI is not installed or not signed in, `oa` will return an error. Check with `oa list` first.
- `--thread` and `--session` are mutually exclusive. Use threads for cross-backend continuity, sessions for single-backend resume.
- Large prompts with full file contents may hit token limits on some models. Point the agent at the file path instead of pasting contents.
