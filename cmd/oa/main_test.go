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
