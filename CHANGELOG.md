# Changelog

All notable changes to oneagent are documented here.

## [0.10.2] - 2026-03-17

### Added

- File locking on thread storage — `LoadThread` acquires a shared lock, `SaveThread` acquires an exclusive lock, preventing corruption when multiple processes (e.g., a bot and a cron job) access the same thread concurrently.
- Turn attribution — `Turn.Source` field and `RunOpts.Source` let callers tag who produced a turn (e.g., `"telegram"`, `"cron-nightly"`, `"ci-pipeline"`).
- Post-run hooks — `RunOpts.OnComplete` and `--on-complete` CLI flag execute a command after a thread turn completes, with the result piped to stdin and `OA_THREAD_ID`, `OA_BACKEND`, `OA_SESSION`, `OA_SOURCE` set as environment variables. Best-effort: hook failures are logged but don't fail the response.

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
