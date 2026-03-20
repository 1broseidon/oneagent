---
id: task-12
title: "Initial embedded skill: mcp-tools"
parentId: epic-2
createdAt: "2026-03-19T19:40:09.762Z"
description: |-
  Write the mcp-tools embedded skill with proper frontmatter per agentskills.io spec. Depends on epic-1 being implemented.

  Create: defaults/skills/mcp-tools/SKILL.md
updatedAt: "2026-03-19T20:37:25.014Z"
completedAt: "2026-03-19T20:37:25.014Z"
---

  name: mcp-tools
  description: Discover and execute MCP server tools via oa mcp list/info/call
  ---

  Body content should cover:
  - When to use: user asks for external tools, integrations, or MCP access
  - Discovery: oa mcp list — shows available servers and tools
  - Inspection: oa mcp info <server> <tool> — get tool schema before calling
  - Execution: oa mcp call <server> <tool> '{"arg": "value"}' — invoke tool
  - Best practice: always list before calling, inspect before first use
  - Keep body under 5000 tokens (spec recommendation)

  Blocked by: epic-1 (MCP subcommands must exist first)
updatedAt: "2026-03-19T19:49:06.878Z"
---
