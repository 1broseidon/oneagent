package main

import (
	"encoding/json"
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

	backends := map[string]oneagent.Backend{
		"summ": {
			Cmd:     []string{"sh", "-c", "printf '%s\n' '{\"result\":\"summary\",\"session\":\"\"}'"},
			Format:  "json",
			Result:  "result",
			Session: "session",
		},
	}
	configDir := filepath.Join(home, "custom")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "backends.json")
	data, err := json.Marshal(backends)
	if err != nil {
		t.Fatalf("marshal backends: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
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
