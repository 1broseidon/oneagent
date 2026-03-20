---
id: task-13
title: "Initial embedded skill: dispatch"
parentId: epic-2
createdAt: "2026-03-19T19:40:09.763Z"
description: |-
  Write the dispatch embedded skill with proper frontmatter per agentskills.io spec.

  Create: defaults/skills/dispatch/SKILL.md
updatedAt: "2026-03-19T20:37:59.796Z"
completedAt: "2026-03-19T20:37:59.796Z"
---

  name: dispatch
  description: Delegate work to other AI agent backends via oa
  ---

  Body content should cover:
  - When to use: user asks to delegate, offload, or run work on another agent/model
  - Basic dispatch: oa -b <backend> '<prompt>'
  - Streaming: oa -b <backend> --jsonl '<prompt>'
  - Piped context: echo '<context>' | oa -b <backend> '<instructions>'
  - Session resume: oa -b <backend> -s <session> '<follow-up>'
  - Thread continuity: oa -b <backend> -t <thread> '<prompt>'
  - Backend selection guidance: when to pick codex vs claude vs opencode vs pi
  - Keep body under 5000 tokens (spec recommendation)
updatedAt: "2026-03-19T19:49:14.369Z"
---
