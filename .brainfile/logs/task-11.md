---
id: task-11
title: Custom skill loading from config dir
parentId: epic-2
createdAt: "2026-03-19T19:40:09.761Z"
description: |-
  Scan custom skills from multiple scopes per agentskills.io spec. Custom skills follow the full spec: multi-scope scanning, cross-client interop, precedence rules. But embedded oa skills (mcp-tools, dispatch, etc.) CANNOT be overridden by custom skills — they are oa's own instruction set.

  Precedence (highest to lowest):
  1. Embedded (built-in oa skills) — always wins, not shadowable
  2. Project-level client-specific: <cwd>/.oneagent/skills/
  3. Project-level cross-client: <cwd>/.agents/skills/
  4. User-level client-specific: ~/.config/oneagent/skills/
  5. User-level cross-client: ~/.agents/skills/

  Discovery rules:
  - Look for subdirectories containing SKILL.md
  - Skip .git/, node_modules/, build artifacts
  - Max depth 4-6 levels, max 2000 dirs scanned
  - Parse YAML frontmatter for name + description
  - Name collisions: embedded always wins (warn if shadowed), then project beats user, within same scope first-found wins with warning
  - Lenient validation per spec: warn on issues, skip only if description missing or YAML unparseable

  Functions:
  - DiscoverSkills(cwd string) []Skill — scan all custom scopes
  - MergeSkills(embedded, discovered) map[string]Skill — embedded wins, then spec precedence

  Files: skills.go, skills_test.go
  Ref: https://agentskills.io/client-implementation/adding-skills-support.md
updatedAt: "2026-03-19T20:37:11.474Z"
completedAt: "2026-03-19T20:37:11.474Z"
---
