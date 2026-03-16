# Changelog

All notable changes to `oneagent` are documented here.

This release history is backfilled from git history. Versions `0.1.0` through `0.8.0` were reconstructed from coherent milestones after the fact rather than being tagged at the time.

## [0.9.0] - 2026-03-16

### Added

- Added a pluggable thread persistence layer through a `Store` interface.
- Added a `Client` type for embedded consumers that own backends plus an optional thread store.
- Added client methods for running, streaming, listing threads, and compacting threads without relying on package-global filesystem behavior.
- Kept package-level helpers as backward-compatible wrappers over the default filesystem store.

### Improved

- Improved the library embedding story for downstream consumers.

## [0.8.0] - 2026-03-16

### Changed

- Made plain text the default CLI output mode.
- Added `--json` for final machine-readable output.
- Added `--jsonl` as a shortcut for `--stream --json`.

### Added

- Added dedicated docs for library usage, backend config, and an end-to-end consumer example.

## [0.7.0] - 2026-03-15

### Added

- Added generic `activity` events to normalized streaming output.

### Changed

- Renamed config loader files from `compact.go` to `config.go`.
- Moved project guidance to `AGENTS.md` and kept compatibility via `CLAUDE.md`.

## [0.6.0] - 2026-03-15

### Added

- Added embedded default backends for Claude, Codex, OpenCode, and Pi.
- Added initial normalized JSONL streaming support to the CLI and library.
- Added first-run behavior that works without a mandatory user-authored backend config.

## [0.5.0] - 2026-03-15

### Added

- Added the compact backend config format and compiler.
- Reduced backend boilerplate by compiling concise JSON definitions into canonical backend structs.

## [0.4.0] - 2026-03-15

### Added

- Added portable thread support for cross-backend conversation continuity.
- Added thread listing, storage, replay, and thread compaction.

## [0.3.0] - 2026-03-15

### Added

- Added a maintainer handoff/project guidance document for agent context.

## [0.2.0] - 2026-03-15

### Changed

- Cleaned up the project for a more public-facing release.

## [0.1.0] - 2026-03-15

### Added

- Initial config-driven multi-agent CLI foundation.
- Basic backend execution, normalized response model, and CLI entrypoint.
