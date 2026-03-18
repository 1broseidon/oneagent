# Getting Started

## Prerequisites

- [Go 1.21+](https://go.dev/dl/)
- At least one supported agent CLI installed and signed in (`claude`, `codex`, `opencode`, or `pi`)

## Install

```sh
go install github.com/1broseidon/oneagent/cmd/oa@latest
```

Or via Homebrew:

```sh
brew install 1broseidon/tap/oa
```

## Quick Start

```sh
# Talk to Claude (the default backend)
oa "explain this codebase"

# Use a different backend
oa -b codex "fix the auth bug"

# Machine-readable JSON output
oa --json "explain this codebase"

# Live text stream
oa --stream "review the repo"

# Specify model and working directory
oa -b pi -m "google/gemini-2.5-pro" -C ~/project "add tests"

# Pipe content as context
git diff | oa -b claude "review these changes"
cat handler.go | oa -b codex "find bugs in this file"
```

Works out of the box if `claude`, `codex`, `opencode`, or `pi` is installed and signed in.

## Supported Backends

| Backend  | CLI            | Session Resume       |
|----------|----------------|----------------------|
| Claude   | `claude`       | `--resume`           |
| Codex    | `codex exec`   | `codex exec resume`  |
| OpenCode | `opencode run` | `--session`          |
| Pi       | `pi`           | `--session`          |

Any CLI that outputs JSON or line-delimited JSON can be added via [config](/reference/config).

## Configuration

`oa` ships with built-in defaults — no config file needed. To override or add backends, create `~/.config/oneagent/backends.json`:

```json
{
  "my-agent": {
    "run": "my-agent --prompt {prompt} --model {model}",
    "format": "json",
    "result": "output.text",
    "session": "session_id"
  }
}
```

See the full [config reference](/reference/config) for all fields.
