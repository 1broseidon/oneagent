package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/1broseidon/oneagent"
)

func TestThreadCompactUsesExplicitConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	thread := &oneagent.Thread{
		ID: "compact-config",
		Turns: []oneagent.Turn{
			{Role: "user", Content: "one"},
			{Role: "assistant", Content: "reply one"},
			{Role: "user", Content: "two"},
			{Role: "assistant", Content: "reply two"},
			{Role: "user", Content: "three"},
			{Role: "assistant", Content: "reply three"},
		},
		NativeSessions: map[string]string{},
	}
	if err := thread.Save(); err != nil {
		t.Fatalf("save thread: %v", err)
	}

	configDir := filepath.Join(home, "custom")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "backends.json")
	// Write a helper script that emits deterministic JSON.
	scriptPath := filepath.Join(home, "summarize.sh")
	os.WriteFile(scriptPath, []byte("#!/bin/sh\necho '{\"result\":\"summary\",\"session\":\"\"}'"), 0o755)

	configJSON := []byte(`{
		"summ": {
			"run": "` + scriptPath + `",
			"format": "json",
			"result": "result",
			"session": "session"
		}
	}`)
	if err := os.WriteFile(configPath, configJSON, 0o644); err != nil {
		t.Fatalf("write backends: %v", err)
	}

	cmd := exec.Command("go", "run", ".", "thread", "compact", "-c", configPath, thread.ID, "-b", "summ")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oa thread compact should honor -c and succeed, got err=%v output=%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "compacted" {
		t.Fatalf("oa thread compact output = %q, want %q", got, "compacted")
	}

	got, err := oneagent.LoadThread(thread.ID)
	if err != nil {
		t.Fatalf("load compacted thread: %v", err)
	}
	if got.Summary != "summary" {
		t.Fatalf("thread summary = %q, want %q", got.Summary, "summary")
	}
}

func TestResolveBackendProgramHandlesEmptyCmd(t *testing.T) {
	name, found := oneagent.ResolveBackendProgram(oneagent.Backend{})
	if name != "(invalid)" || found {
		t.Fatalf("ResolveBackendProgram(empty) = %q, %v; want %q, false", name, found, "(invalid)")
	}
}

func TestListUsesEmbeddedDefaultsWithoutUserConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := exec.Command("go", "run", ".", "list")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oa list should succeed with embedded defaults, got err=%v output=%s", err, out)
	}

	for _, name := range []string{"claude", "codex", "opencode", "pi"} {
		if !strings.Contains(string(out), name) {
			t.Fatalf("oa list output missing %q:\n%s", name, out)
		}
	}
}

func TestSkillsListUsesEmbeddedDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := exec.Command("go", "run", ".", "skills", "list")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oa skills list should succeed, got err=%v output=%s", err, out)
	}

	text := string(out)
	for _, name := range []string{"dispatch", "mcp-tools"} {
		if !strings.Contains(text, name) {
			t.Fatalf("oa skills list output missing %q:\n%s", name, out)
		}
	}
	if !strings.Contains(text, "(built-in)") {
		t.Fatalf("oa skills list output missing built-in source:\n%s", out)
	}
}

func TestSkillsShowPrintsWrappedBody(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := exec.Command("go", "run", ".", "skills", "show", "dispatch")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oa skills show should succeed, got err=%v output=%s", err, out)
	}

	text := string(out)
	if !strings.Contains(text, `<skill_content name="dispatch">`) {
		t.Fatalf("oa skills show output missing wrapper:\n%s", out)
	}
	if !strings.Contains(text, "# Dispatch") {
		t.Fatalf("oa skills show output missing body:\n%s", out)
	}
	if !strings.Contains(text, "Skill directory: defaults/skills/dispatch") {
		t.Fatalf("oa skills show output missing skill directory:\n%s", out)
	}
	if !strings.Contains(text, "<skill_resources>") || !strings.Contains(text, "</skill_content>") {
		t.Fatalf("oa skills show output missing resource wrapper:\n%s", out)
	}
}

func TestSkillsShowMissingSkillFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd := exec.Command("go", "run", ".", "skills", "show", "missing-skill")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("oa skills show missing skill should fail, output=%s", out)
	}
	if !strings.Contains(string(out), "skill not found: missing-skill") {
		t.Fatalf("oa skills show missing skill output = %q", out)
	}
}

func TestSkillsListHonorsCWDForProjectSkills(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)

	writeSkillFile(t, filepath.Join(project, ".agents", "skills", "my-skill"), `---
name: my-skill
description: Project custom skill.
---

# My Skill
`)

	cmd := exec.Command("go", "run", ".", "skills", "list", "-C", project)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oa skills list -C should succeed, got err=%v output=%s", err, out)
	}

	text := string(out)
	if !strings.Contains(text, "my-skill") {
		t.Fatalf("oa skills list -C output missing custom skill:\n%s", out)
	}
	if !strings.Contains(text, "(.agents/skills/)") {
		t.Fatalf("oa skills list -C output missing project source:\n%s", out)
	}
}

func TestSkillsShowListsCustomResources(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)

	skillDir := filepath.Join(project, ".agents", "skills", "resource-skill")
	writeSkillFile(t, skillDir, `---
name: resource-skill
description: Skill with bundled resources.
---

# Resource Skill
`)
	if err := os.MkdirAll(filepath.Join(skillDir, "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "scripts", "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	cmd := exec.Command("go", "run", ".", "skills", "show", "resource-skill", "-C", project)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oa skills show -C should succeed, got err=%v output=%s", err, out)
	}

	text := string(out)
	if !strings.Contains(text, "<file>scripts/run.sh</file>") {
		t.Fatalf("oa skills show output missing bundled resource:\n%s", out)
	}
	if !strings.Contains(text, "Skill directory: "+skillDir) {
		t.Fatalf("oa skills show output missing absolute directory:\n%s", out)
	}
}

func TestListHonorsExplicitConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(home, "backends.json")
	configJSON := []byte(`{
		"only": {
			"run": "only-agent {prompt}",
			"format": "json",
			"result": "result",
			"session": "session"
		}
	}`)
	if err := os.WriteFile(configPath, configJSON, 0o644); err != nil {
		t.Fatalf("write backends: %v", err)
	}

	cmd := exec.Command("go", "run", ".", "list", "-c", configPath)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oa list -c should succeed, got err=%v output=%s", err, out)
	}

	if !strings.Contains(string(out), "only") {
		t.Fatalf("oa list -c output missing explicit backend:\n%s", out)
	}
	if strings.Contains(string(out), "claude") {
		t.Fatalf("oa list -c should not merge embedded defaults:\n%s", out)
	}
}

func writeSkillFile(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill %s: %v", dir, err)
	}
}

func TestDefaultPromptOutputsText(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	scriptPath := filepath.Join(home, "resp.sh")
	script := "#!/bin/sh\n" +
		"printf '%s\n' '{\"result\":\"hello\",\"session\":\"sess-1\"}'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	configPath := filepath.Join(home, "backends.json")
	configJSON := []byte(`{
		"s": {
			"run": "` + scriptPath + `",
			"format": "json",
			"result": "result",
			"session": "session"
		}
	}`)
	if err := os.WriteFile(configPath, configJSON, 0o644); err != nil {
		t.Fatalf("write backends: %v", err)
	}

	cmd := exec.Command("go", "run", ".", "-b", "s", "-c", configPath, "hi")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oa default text output should succeed, got err=%v output=%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "hello" {
		t.Fatalf("oa default output = %q, want %q", got, "hello")
	}
}

func TestJSONFlagOutputsJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	scriptPath := filepath.Join(home, "resp.sh")
	script := "#!/bin/sh\n" +
		"printf '%s\n' '{\"result\":\"hello\",\"session\":\"sess-1\"}'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	configPath := filepath.Join(home, "backends.json")
	configJSON := []byte(`{
		"s": {
			"run": "` + scriptPath + `",
			"format": "json",
			"result": "result",
			"session": "session"
		}
	}`)
	if err := os.WriteFile(configPath, configJSON, 0o644); err != nil {
		t.Fatalf("write backends: %v", err)
	}

	cmd := exec.Command("go", "run", ".", "--json", "-b", "s", "-c", configPath, "hi")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oa --json should succeed, got err=%v output=%s", err, out)
	}
	if !strings.Contains(string(out), `"result": "hello"`) {
		t.Fatalf("oa --json output missing result:\n%s", out)
	}
}

func TestStreamOutputsTextByDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	scriptPath := filepath.Join(home, "stream.sh")
	script := "#!/bin/sh\n" +
		"printf '%s\n' '{\"type\":\"session\",\"sid\":\"sess-1\"}'\n" +
		"printf '%s\n' '{\"type\":\"activity\",\"tool\":{\"name\":\"Read\",\"path\":\"README.md\"}}'\n" +
		"printf '%s\n' '{\"type\":\"delta\",\"data\":{\"text\":\"hello\"}}'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	configPath := filepath.Join(home, "backends.json")
	configJSON := []byte(`{
		"s": {
			"run": "` + scriptPath + `",
			"format": "jsonl",
			"activity": "{tool.name} {tool.path}",
			"activity_when": "type=activity",
			"result": "data.text",
			"result_when": "type=delta",
			"result_append": true,
			"session": "sid",
			"session_when": "type=session"
		}
	}`)
	if err := os.WriteFile(configPath, configJSON, 0o644); err != nil {
		t.Fatalf("write backends: %v", err)
	}

	cmd := exec.Command("go", "run", ".", "--stream", "-b", "s", "-c", configPath, "hi")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oa --stream text should succeed, got err=%v output=%s", err, out)
	}

	text := string(out)
	if !strings.Contains(text, "[activity] Read README.md") {
		t.Fatalf("stream text output missing activity line:\n%s", text)
	}
	if !strings.Contains(text, "hello") {
		t.Fatalf("stream text output missing assistant text:\n%s", text)
	}
}

func TestStreamOutputsNormalizedJSONL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	scriptPath := filepath.Join(home, "stream.sh")
	script := "#!/bin/sh\n" +
		"printf '%s\n' '{\"type\":\"session\",\"sid\":\"sess-1\"}'\n" +
		"printf '%s\n' '{\"type\":\"activity\",\"tool\":{\"name\":\"Read\",\"path\":\"README.md\"}}'\n" +
		"printf '%s\n' '{\"type\":\"delta\",\"data\":{\"text\":\"hello\"}}'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	configPath := filepath.Join(home, "backends.json")
	configJSON := []byte(`{
		"s": {
			"run": "` + scriptPath + `",
			"format": "jsonl",
			"activity": "{tool.name} {tool.path}",
			"activity_when": "type=activity",
			"result": "data.text",
			"result_when": "type=delta",
			"result_append": true,
			"session": "sid",
			"session_when": "type=session"
		}
	}`)
	if err := os.WriteFile(configPath, configJSON, 0o644); err != nil {
		t.Fatalf("write backends: %v", err)
	}

	cmd := exec.Command("go", "run", ".", "--stream", "--json", "-b", "s", "-c", configPath, "hi")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oa --stream --json should succeed, got err=%v output=%s", err, out)
	}

	text := string(out)
	if !strings.Contains(text, `"type":"session"`) {
		t.Fatalf("stream output missing session event:\n%s", text)
	}
	if !strings.Contains(text, `"type":"activity"`) {
		t.Fatalf("stream output missing activity event:\n%s", text)
	}
	if !strings.Contains(text, `"type":"delta"`) {
		t.Fatalf("stream output missing delta event:\n%s", text)
	}
	if !strings.Contains(text, `"type":"done"`) {
		t.Fatalf("stream output missing done event:\n%s", text)
	}
}

func TestJSONLAliasOutputsNormalizedJSONL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	scriptPath := filepath.Join(home, "stream.sh")
	script := "#!/bin/sh\n" +
		"printf '%s\n' '{\"type\":\"session\",\"sid\":\"sess-1\"}'\n" +
		"printf '%s\n' '{\"type\":\"activity\",\"tool\":{\"name\":\"Read\",\"path\":\"README.md\"}}'\n" +
		"printf '%s\n' '{\"type\":\"delta\",\"data\":{\"text\":\"hello\"}}'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	configPath := filepath.Join(home, "backends.json")
	configJSON := []byte(`{
		"s": {
			"run": "` + scriptPath + `",
			"format": "jsonl",
			"activity": "{tool.name} {tool.path}",
			"activity_when": "type=activity",
			"result": "data.text",
			"result_when": "type=delta",
			"result_append": true,
			"session": "sid",
			"session_when": "type=session"
		}
	}`)
	if err := os.WriteFile(configPath, configJSON, 0o644); err != nil {
		t.Fatalf("write backends: %v", err)
	}

	cmd := exec.Command("go", "run", ".", "--jsonl", "-b", "s", "-c", configPath, "hi")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oa --jsonl should succeed, got err=%v output=%s", err, out)
	}

	text := string(out)
	if !strings.Contains(text, `"type":"session"`) || !strings.Contains(text, `"type":"done"`) {
		t.Fatalf("oa --jsonl output missing expected stream events:\n%s", text)
	}
}

func TestVersionFlagOutputsBuildVersion(t *testing.T) {
	cmd := exec.Command("go", "run", "-ldflags", "-X main.version=v1.2.3", ".", "--version")
	cmd.Dir = "."

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("oa --version should succeed, got err=%v output=%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "oa v1.2.3" {
		t.Fatalf("oa --version output = %q, want %q", got, "oa v1.2.3")
	}
}

func TestLoadRunContextIncludesHooks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	_, opts := loadRunContext(cliOpts{
		backend: "claude",
		thread:  "daily",
		preRun:  "echo before",
		postRun: "echo after",
		prompt:  []string{"summarize", "today"},
	})

	if opts.PreRunCmd != "echo before" {
		t.Fatalf("pre_run = %q, want %q", opts.PreRunCmd, "echo before")
	}
	if opts.PostRunCmd != "echo after" {
		t.Fatalf("post_run = %q, want %q", opts.PostRunCmd, "echo after")
	}
	if opts.ThreadID != "daily" {
		t.Fatalf("thread_id = %q, want %q", opts.ThreadID, "daily")
	}
	if opts.Prompt != "summarize today" {
		t.Fatalf("prompt = %q, want %q", opts.Prompt, "summarize today")
	}
}
