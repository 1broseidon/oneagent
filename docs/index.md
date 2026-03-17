---
layout: home
hero:
  name: oneagent
  text: Config-driven multi-agent CLI
  tagline: One interface for Claude, Codex, OpenCode, Pi, and any future agent. Normalized output, portable threads, zero code to add a backend.
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/1broseidon/oneagent
features:
  - title: Normalized Output
    details: Every backend returns the same JSON shape. Stream events, parse results, switch models — your code never changes.
  - title: Portable Threads
    details: Start a conversation on Claude, continue on Codex, summarize on Pi. Thread history follows across backends with file locking for concurrency.
  - title: Zero-Code Backends
    details: Adding a new agent is a JSON config edit. Define the CLI command, map the output fields, done.
  - title: Agent-as-Tool
    details: Dispatch work from one agent to another. The outer agent calls oa, gets normalized results back, chains follow-ups across models.
---
