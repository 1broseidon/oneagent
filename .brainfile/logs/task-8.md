---
id: task-8
title: Embed default skill markdown files
parentId: epic-2
createdAt: "2026-03-19T19:40:09.757Z"
description: "Create defaults/skills/ directory with embedded SKILL.md files using //go:embed. Each SKILL.md has YAML frontmatter per agentskills.io spec:"
updatedAt: "2026-03-19T20:30:41.161Z"
completedAt: "2026-03-19T20:30:41.161Z"
---

  name: skill-name
  description: One-line description for catalog disclosure
  ---

  Body content here...

  Implementation:
  - New skills.go file with //go:embed defaults/skills/*/SKILL.md
  - Skill struct: Name, Description, Body, Location, Source (embedded|user|project)
  - ParseSkill(data []byte) — extract frontmatter (name, description) and body
  - LoadEmbeddedSkills() returning map[string]Skill
  - Lenient parsing: warn on issues, skip only if description missing or YAML unparseable
  - Store name, description, body in memory for catalog and activation

  Files: skills.go, skills_test.go
  Ref: https://agentskills.io/client-implementation/adding-skills-support.md
updatedAt: "2026-03-19T19:48:24.771Z"
---
