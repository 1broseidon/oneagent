package oneagent

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestLoadEmbeddedSkills(t *testing.T) {
	skills, err := LoadEmbeddedSkills()
	if err != nil {
		t.Fatalf("LoadEmbeddedSkills: %v", err)
	}

	got := MergeSkills(skills, nil)
	for _, name := range []string{"dispatch", "mcp-tools"} {
		skill, ok := got[name]
		if !ok {
			t.Fatalf("embedded skill %q missing from %v", name, skillNames(got))
		}
		if skill.Description == "" {
			t.Fatalf("embedded skill %q missing description", name)
		}
		if skill.Body == "" {
			t.Fatalf("embedded skill %q missing body", name)
		}
	}
}

func TestDiscoverSkillsScansCustomScopes(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)

	writeSkill(t, filepath.Join(project, ".oneagent", "skills", "project-native"), "project-native", "Project client skill.", "project body")
	writeSkill(t, filepath.Join(project, ".agents", "skills", "project-shared"), "project-shared", "Project shared skill.", "shared body")
	writeSkill(t, filepath.Join(home, ".config", "oneagent", "skills", "user-native"), "user-native", "User client skill.", "user body")
	writeSkill(t, filepath.Join(home, ".agents", "skills", "user-shared"), "user-shared", "User shared skill.", "shared user body")

	skills := MergeSkills(nil, DiscoverSkills(project))
	for name, source := range map[string]string{
		"project-native": ".oneagent/skills/",
		"project-shared": ".agents/skills/",
		"user-native":    "~/.config/oneagent/skills/",
		"user-shared":    "~/.agents/skills/",
	} {
		skill, ok := skills[name]
		if !ok {
			t.Fatalf("discovered skill %q missing from %v", name, skillNames(skills))
		}
		if skill.Source != source {
			t.Fatalf("skill %q source = %q, want %q", name, skill.Source, source)
		}
		if !filepath.IsAbs(skill.Location) {
			t.Fatalf("skill %q location should be absolute, got %q", name, skill.Location)
		}
	}
}

func TestMergeSkillsUsesExpectedPrecedence(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)

	writeSkill(t, filepath.Join(project, ".oneagent", "skills", "dupe-project"), "dupe", "Project client winner.", "project client")
	writeSkill(t, filepath.Join(project, ".agents", "skills", "dupe-shared"), "dupe", "Project shared loser.", "project shared")
	writeSkill(t, filepath.Join(home, ".config", "oneagent", "skills", "dupe-user"), "dupe", "User client loser.", "user client")
	writeSkill(t, filepath.Join(home, ".agents", "skills", "dupe-user-shared"), "dupe", "User shared loser.", "user shared")
	writeSkill(t, filepath.Join(home, ".agents", "skills", "dispatch"), "dispatch", "Custom dispatch should lose.", "custom dispatch")

	embedded, err := LoadEmbeddedSkills()
	if err != nil {
		t.Fatalf("LoadEmbeddedSkills: %v", err)
	}

	skills := MergeSkills(embedded, DiscoverSkills(project))
	if got := skills["dupe"].Source; got != ".oneagent/skills/" {
		t.Fatalf("dupe source = %q, want %q", got, ".oneagent/skills/")
	}
	if got := skills["dupe"].Description; got != "Project client winner." {
		t.Fatalf("dupe description = %q, want %q", got, "Project client winner.")
	}
	if got := skills["dispatch"].Source; got != skillSourceBuiltIn {
		t.Fatalf("dispatch source = %q, want %q", got, skillSourceBuiltIn)
	}
}

func TestDiscoverSkillsRepairsColonDescriptions(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)

	raw := `---
name: pdf-tools
description: Use this skill when: the user asks about PDFs
---

# PDF Tools
`
	writeSkillRaw(t, filepath.Join(project, ".agents", "skills", "pdf-tools"), raw)

	skills := MergeSkills(nil, DiscoverSkills(project))
	skill, ok := skills["pdf-tools"]
	if !ok {
		t.Fatalf("pdf-tools missing from %v", skillNames(skills))
	}
	if got := skill.Description; got != "Use this skill when: the user asks about PDFs" {
		t.Fatalf("pdf-tools description = %q", got)
	}
}

func TestDiscoverSkillsSkipsMissingDescriptionAndKeepsFirstDuplicate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)

	writeSkill(t, filepath.Join(project, ".agents", "skills", "alpha"), "dupe", "First duplicate wins.", "first")
	writeSkill(t, filepath.Join(project, ".agents", "skills", "beta"), "dupe", "Second duplicate loses.", "second")
	writeSkillRaw(t, filepath.Join(project, ".agents", "skills", "missing-description"), `---
name: missing-description
---

missing description
`)

	skills := MergeSkills(nil, DiscoverSkills(project))
	skill, ok := skills["dupe"]
	if !ok {
		t.Fatalf("dupe missing from %v", skillNames(skills))
	}
	if got := skill.Description; got != "First duplicate wins." {
		t.Fatalf("dupe description = %q, want %q", got, "First duplicate wins.")
	}
	if _, ok := skills["missing-description"]; ok {
		t.Fatalf("missing-description should have been skipped")
	}
}

func TestBuildSkillCatalogSortedAndOmittedWhenEmpty(t *testing.T) {
	if got := BuildSkillCatalog(nil); got != "" {
		t.Fatalf("BuildSkillCatalog(nil) = %q, want empty string", got)
	}

	catalog := BuildSkillCatalog(map[string]Skill{
		"zeta":  {Name: "zeta", Description: "Last skill."},
		"alpha": {Name: "alpha", Description: "First skill."},
	})
	if !strings.Contains(catalog, "Available skills:") {
		t.Fatalf("catalog missing header: %q", catalog)
	}
	if strings.Index(catalog, "- alpha: First skill.") > strings.Index(catalog, "- zeta: Last skill.") {
		t.Fatalf("catalog should be sorted by name: %q", catalog)
	}
}

func skillNames(skills map[string]Skill) []string {
	names := make([]string, 0, len(skills))
	for name := range skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func writeSkill(t *testing.T, dir, name, description, body string) {
	t.Helper()
	writeSkillRaw(t, dir, skillDoc(name, description, body))
}

func writeSkillRaw(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", dir, err)
	}
}

func skillDoc(name, description, body string) string {
	return "---\n" +
		"name: " + name + "\n" +
		"description: " + description + "\n" +
		"---\n\n" +
		body + "\n"
}
