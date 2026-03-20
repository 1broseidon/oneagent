---
id: epic-1
title: MCP tool bridge
type: epic
column: todo
position: 0
priority: high
tags:
  - mcp
  - tools
  - feature
createdAt: "2026-03-19T19:37:10.967Z"
---

## Description
Add oa mcp subcommand that provides list/info/call for MCP servers, giving all backends unified tool access without per-backend MCP config. Config lives at ~/.config/oneagent/mcp.json. Agents discover tools dynamically via shell commands, avoiding context window bloat from static tool schemas.
