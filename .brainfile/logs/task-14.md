---
id: task-14
title: System prompt field for skill discovery injection
parentId: epic-2
createdAt: "2026-03-19T19:40:09.765Z"
description: |-
  Inject skill catalog into system prompt at session start (tier 1: progressive disclosure). This is the ~50-100 tokens per skill that tells the agent what's available.

  Injected text format:
    The following skills provide specialized instructions for specific tasks.
    When a task matches a skill description, run 'oa skills show <name>' to load its full instructions.

    Available skills:
    - mcp-tools: MCP tool discovery and execution
    - dispatch: Delegate work to other backends

  Implementation:
  - In buildCmd or invoke, after loading skills, build catalog string from name+description
  - Append catalog to system_prompt field (or prepend, before user system_prompt)
  - Only inject when skills exist; omit entirely when no skills discovered
  - Catalog is built from merged skills (embedded + custom), respecting precedence

  Files: oneagent.go or skills.go
  Ref: https://agentskills.io/client-implementation/adding-skills-support.md (Step 3)
updatedAt: "2026-03-19T20:43:58.590Z"
completedAt: "2026-03-19T20:43:58.590Z"
---
