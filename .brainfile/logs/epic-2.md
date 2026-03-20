---
id: epic-2
title: Embedded skill system
type: epic
priority: high
tags:
  - skills
  - system-prompt
  - feature
createdAt: "2026-03-19T19:40:09.753Z"
updatedAt: "2026-03-19T20:44:00.065Z"
completedAt: "2026-03-19T20:44:00.065Z"
---

## Description
Add oa skills subcommand with embedded skill files (go:embed) for teaching agents how to use oa capabilities. Embedded skills ship tamper-proof in the binary. Custom skills extend via ~/.config/oneagent/skills/<name>/SKILL.md. System prompt just tells agents to run oa skills list / oa skills show <name> for dynamic discovery. Same layering pattern as backends.json.

## Child Tasks
Summary: 7/7 children completed.
- task-10: oa skills show subcommand (completed)
- task-11: Custom skill loading from config dir (completed)
- task-12: Initial embedded skill: mcp-tools (completed)
- task-13: Initial embedded skill: dispatch (completed)
- task-14: System prompt field for skill discovery injection (completed)
- task-8: Embed default skill markdown files (completed)
- task-9: oa skills list subcommand (completed)
