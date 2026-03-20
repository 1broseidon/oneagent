---
id: task-10
title: oa skills show subcommand
parentId: epic-2
createdAt: "2026-03-19T19:40:09.760Z"
description: |-
  Add 'oa skills show <name>' subcommand. Returns skill content for agent activation (tier 2 of progressive disclosure).

  Behavior:
  - Look up skill by name in merged skill map
  - Strip YAML frontmatter, return markdown body only
  - Wrap in structured tags for context management:
    <skill_content name="skill-name">
    [body]
    Skill directory: /path/to/skill/dir
    <skill_resources>
      <file>scripts/foo.py</file>
    </skill_resources>
    </skill_content>
  - List bundled resources (files in skill dir besides SKILL.md) but do NOT read them
  - Exit 1 with 'skill not found: <name>' to stderr if missing

  Files: cmd/oa/main.go
  Ref: https://agentskills.io/client-implementation/adding-skills-support.md (Step 4)
updatedAt: "2026-03-19T20:33:23.779Z"
completedAt: "2026-03-19T20:33:23.779Z"
---
