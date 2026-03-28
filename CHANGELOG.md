# Changelog

All notable changes to oneagent are documented here.

## [0.12.2] - 2026-03-28

### Added

- **Native system prompt routing** — new `{system}` template variable in backend run commands. When a backend's command template includes `{system}`, the system prompt is passed via the CLI's native flag (e.g. `--append-system-prompt`, `--system`) instead of being prepended to the user message. This gives models higher-weight system-level instructions and frees up user-message context.
- Default backends updated: `claude` uses `--append-system-prompt {system}`, `pi` uses `--append-system-prompt {system}`, `opencode` uses `--system {system}`. Codex and Gemini fall back to prompt prepend (no CLI flag available yet).

## [0.12.1] - 2026-03-28

### Added

- **Preflight checks** — `PreflightCheck` and `PreflightCheckBackend` validate that a backend CLI exists and is ready before any work is queued. Catches missing binaries and auth issues instantly instead of failing minutes later during execution.
- **`Probe` field** on `Backend` — optional fast command (e.g. `claude --version`) that runs during preflight to verify the backend is functional beyond just being installed. Configured via the `"probe"` key in `backends.json`.
- Default probe commands added for all built-in backends: `claude`, `codex`, `opencode`, `pi`, `gemini`.

## [0.11.13] - 2026-03-24

### Added

- `Warnings` field on `Response` — populated from stderr when the backend succeeds but emits diagnostics.

### Fixed

- Backend errors now surface descriptive messages from stderr instead of generic `"exit status 1"`, enabling agent self-healing on invalid parameters.

## [0.11.12] - 2026-03-23

### Fixed

- Deduplicate concurrent thread turn recording to prevent duplicate messages when multiple writers (e.g. synthesis and main conversation) write to the same thread simultaneously.

## [0.11.11] - 2026-03-22

### Added

- Streaming run telemetry now includes `run_id` and `ts` on normalized events, plus synthetic `start` and periodic `heartbeat` events emitted by the library while a backend process is alive.

### Changed

- JSON-format backends now use the same start/heartbeat-aware process execution path as JSONL streaming runs, improving observability without changing the public `Run*` APIs.

## [0.11.10] - 2026-03-21

### Fixed

- Empty inline-assignment template args like `-c model_reasoning_effort={thinking}` now drop cleanly when the placeholder is unset instead of leaving invalid backend-specific fragments behind.

## [0.11.9] - 2026-03-20

### Fixed

- Backends that exit non-zero but produce a valid result (e.g. Cline exits 1 on success) are now treated as successful instead of overwriting the result with an error.

## [0.11.8] - 2026-03-20

### Added

- `Thinking` field on `RunOpts` and `--thinking <level>` CLI flag for controlling thinking/reasoning effort per invocation.
- `{thinking}` template variable in backend configs — drops silently when unset.
- Thinking support in Claude (`--effort`), Codex (`-c model_reasoning_effort=`), and Pi (`--thinking`) embedded configs.

## [0.11.7] - 2026-03-20

### Added

- `RunContext` and `RunStreamContext` methods on `Client` and package level for cancellable backend invocations via `context.Context`.
- `ExitCode` and `Stderr` fields on `Response` for richer error diagnostics.
- Negative array index support in `jsonGet` (e.g., `content.-1.text` for last element).

### Fixed

- Pi backend empty result when model includes thinking tokens before text content — now uses `turn_end` with `content.-1.text`.
- Session backfill on resume — `resp.Session` is populated from `opts.SessionID` when no session event arrives from the backend.

## [0.11.6] - 2026-03-20

### Fixed

- Empty successful backend results are now preserved as empty strings instead of being rewritten to `Done — nothing to report.`

## [0.11.5] - 2026-03-20

### Added

- `LoadBackendsWithOptions` and `LoadOptions` for consumers that want embedded defaults plus an app-owned override path instead of `~/.config/oneagent/backends.json`.

### Fixed

- Pi backend result selection now uses the final assistant message while keeping `text_delta` streaming separate, preventing intermediate narration from being folded into the final result.

## [0.11.4] - 2026-03-20

### Added

- Embedded Gemini CLI backend with stream-json output parsing.

### Changed

- Removed `--skills` references from `oa-dispatch` skill.

## [0.11.3] - 2026-03-19

### Added

- golangci-lint config (`.golangci.yml`) and pre-commit hook.

### Changed

- Cyclomatic complexity threshold relaxed from 10 to 15.
- `buildCmd` refactored into smaller helpers for readability.

### Fixed

- Unchecked `f.Close()` error in thread loading.

## [0.11.2] - 2026-03-19

### Fixed

- Activity events for claude, opencode, and pi backends now include tool context (description, command, title) instead of just the tool name.

### Changed

- `oa-dispatch` skill uses pipe form for all prompts.

## [0.11.1] - 2026-03-18

### Fixed

- **Security**: Thread ID path traversal — IDs containing `../` or path separators are now rejected, preventing writes outside the thread directory.
- Thread saves are now atomic — writes to a temp file then renames, preventing corruption on crash.
- `cwd` check in `buildCmd` now uses the active template (run or resume) instead of always checking the run template.
- `PostRun` callback now receives the original user prompt, not the replay-expanded version with thread context.
- Scanner errors in JSONL parsing are now always logged, even when the backend also exits non-zero.

### Added

- Example scripts: multi-agent conversation, self-healing tests, parallel review, daily digest.
- `converse.sh` and `multi-review.sh` bundled in the `oa-dispatch` agent skill.

## [0.11.0] - 2026-03-18

### Added

- Pre/post hook system — run commands or Go callbacks before and after agent execution. Config-level hooks fire on every backend invocation, CLI hooks (`--pre-run`/`--post-run`) are per-invocation. Both layers stack.
- Library-level hook callbacks: `PreRun func(*RunOpts) error` can modify options or abort, `PostRun func(*HookContext)` for side effects.
- `HookContext` type exposing both `RunOpts` and `Response` to post-run callbacks.
- Internal calls (e.g., `CompactThread`) bypass hooks via `runDirect()`.

### Removed

- Legacy completion hook field/flag — replaced by the general `PostRunCmd`/`--post-run` hook.

## [0.10.7] - 2026-03-18

### Added

- `prompt_stdin` config field — pass prompts via stdin instead of command-line arguments, keeping prompt content out of `ps` output. All four embedded backends now use this by default.

## [0.10.6] - 2026-03-17

### Added

- Stdin pipe support — pipe content into `oa` as context, with optional positional args as instructions. Examples: `cat file.go | oa -b claude "review this"`, `git diff | oa -b claude "explain these changes"`.
- `oa backends` alias for `oa list`.
- Docs site now auto-rebuilds after every release so the version badge stays current.

## [0.10.5] - 2026-03-17

### Added

- `paths` config field — additional directories to search when a backend CLI isn't in `$PATH`. Supports `~` expansion. Embedded defaults ship with known install locations for each backend.
- Troubleshooting doc with PATH and config-based fixes for missing backends.
- Improved error message when a backend CLI is not found, with a link to the troubleshooting guide.

### Changed

- `oa list` output cleaned up: no redundant program name or format column, only shows model when explicitly set, marks unavailable backends with `(not installed)`.

## [0.10.4] - 2026-03-17

### Changed

- `oa list` now shows `(not installed)` for backends whose CLI is not found in PATH.
- Switched Homebrew distribution from cask to formula — no more Gatekeeper issues on macOS.
- Install with `brew install 1broseidon/tap/oa`.

## [0.10.3] - 2026-03-17

### Fixed

- Codex backend now uses `--dangerously-bypass-approvals-and-sandbox` instead of `--full-auto` — bubblewrap sandbox fails on some kernels, blocking all writes.

## [0.10.2] - 2026-03-17

### Added

- File locking on thread storage — `LoadThread` acquires a shared lock, `SaveThread` acquires an exclusive lock, preventing corruption when multiple processes (e.g., a bot and a cron job) access the same thread concurrently.
- Turn attribution — `Turn.Source` field and `RunOpts.Source` let callers tag who produced a turn (e.g., `"telegram"`, `"cron-nightly"`, `"ci-pipeline"`).
- Post-run hooks — `RunOpts.PostRunCmd` and `--post-run` execute a command after a thread turn completes, with the result piped to stdin and `OA_THREAD_ID`, `OA_BACKEND`, `OA_SESSION`, `OA_SOURCE` set as environment variables. Best-effort: hook failures are logged but don't fail the response.

## [0.10.1] - 2026-03-16

### Added

- Activity events for the OpenCode backend — tool use (file reads, patches) now appears in JSONL streaming output.
- `oa-dispatch` agent skill for dispatching tasks to other agents, following the [agentskills.io](https://agentskills.io) spec. Install with `npx skills add 1broseidon/oneagent --skill oa-dispatch`.

### Improved

- Rewrote all public-facing docs for clarity and correctness: README, library guide, config reference, integration example, and changelog.
- README now leads with a clear value proposition and includes prerequisites.
- Library docs updated to use the Client-based API introduced in 0.9.0.
- Integration example replaced with a complete, runnable `main.go`.
- Config docs include all four backend examples with verified minimum flags for non-interactive execution.

### Fixed

- Claude backend now includes `--dangerously-skip-permissions` — required for write and bash operations since `-p` mode only permits reads.

## [0.10.0] - 2026-03-16

### Added

- GoReleaser configuration and a tag-driven GitHub Actions release workflow.
- Homebrew tap publishing configuration targeting `1broseidon/homebrew-tap`.
- `oa --version` and `oa version` for release verification and package-manager smoke tests.

## [0.9.0] - 2026-03-16

### Added

- Pluggable thread storage through a `Store` interface — use the built-in filesystem store or bring your own.
- `Client` type for applications that embed oneagent, with methods for running, streaming, and managing threads.

## [0.8.0] - 2026-03-16

### Changed

- Plain text is now the default CLI output mode. Use `--json` for machine-readable output.
- Added `--jsonl` as a shortcut for `--stream --json`.

### Added

- Dedicated docs for library usage, backend config, and integration examples.

## [0.7.0] - 2026-03-15

### Added

- Generic `activity` events in normalized streaming output, showing what the agent is doing (e.g., reading a file, running a command).

## [0.6.0] - 2026-03-15

### Added

- Embedded default backends for Claude, Codex, OpenCode, and Pi — works on first run with no config file.
- Normalized JSONL streaming in both the CLI and Go library.

## [0.5.0] - 2026-03-15

### Added

- Compact backend config format — define backends in concise JSON instead of writing Go code.

## [0.4.0] - 2026-03-15

### Added

- Portable threads for cross-backend conversation continuity.
- Thread listing, inspection, and compaction commands.

## [0.2.0] - 2026-03-15

### Changed

- Improved public API surface and documentation.

## [0.1.0] - 2026-03-15

### Added

- Initial release: config-driven multi-agent CLI with normalized JSON output and session resume.
