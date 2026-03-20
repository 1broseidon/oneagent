---
id: task-9
title: oa skills list subcommand
parentId: epic-2
createdAt: "2026-03-19T19:40:09.758Z"
description: |-
  Add 'oa skills list' subcommand. Shows all discovered skills (embedded + custom) with source and scope.

  Output format:
    mcp-tools    MCP tool discovery and execution    (built-in)
    dispatch     Delegate work to other backends      (built-in)
    pdf-tools    Extract and process PDF files        (~/.agents/skills/)
    my-skill     Custom project skill                 (.agents/skills/)

  Implementation:
  - Add 'skills' case in main() command dispatch
  - Accepts optional -C/--cwd for project scope scanning
  - Calls oneagent.DiscoverSkills + MergeSkills with embedded
  - Sorted output: name, description (truncated), source

  Files: cmd/oa/main.go
updatedAt: "2026-03-19T20:31:49.708Z"
completedAt: "2026-03-19T20:31:49.708Z"
---
