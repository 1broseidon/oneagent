# Agent-as-Tool

`oa` works as a dispatch layer for agents that want to delegate work to other agents.

## How It Works

An outer agent (e.g., Claude Code) runs `oa` as a background task to send a targeted edit to a different model, then inspects the diff when it's done:

```sh
# From inside an agent session — dispatch a file edit to gpt-5.4 via Pi
oa -b pi -m openai-codex/gpt-5.4 \
  "Edit internal/auth/handler.go: add rate limiting to Login" --jsonl

# Verify the result
git diff internal/auth/handler.go
```

The normalized output means the outer agent can parse results from any backend without special handling. Portable threads let you chain follow-ups across models.

## Agent Skill

An agent skill is included for agents that support the [Agent Skills](https://agentskills.io) format:

```sh
npx skills add 1broseidon/oneagent --skill oa-dispatch
```

This gives supporting agents structured knowledge about how to dispatch work through `oa`, including prompt construction, backend selection, threading, and output parsing.
