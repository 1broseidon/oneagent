package oneagent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Turn is a single conversation turn stored in a thread.
type Turn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Backend string `json:"backend"`
	TS      string `json:"ts"`
}

// Thread is a portable conversation that can span multiple backends.
type Thread struct {
	ID             string            `json:"id"`
	Summary        string            `json:"summary,omitempty"`
	Turns          []Turn            `json:"turns"`
	NativeSessions map[string]string `json:"native_sessions,omitempty"`
}

// ThreadDir returns the directory for thread storage.
func ThreadDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "oneagent", "threads")
}

// LoadThread reads a thread from disk. A missing file returns an empty thread.
func LoadThread(id string) (*Thread, error) {
	path := filepath.Join(ThreadDir(), id+".json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Thread{ID: id, NativeSessions: map[string]string{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var t Thread
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("corrupt thread %s: %w", id, err)
	}
	if t.NativeSessions == nil {
		t.NativeSessions = map[string]string{}
	}
	return &t, nil
}

// Save writes the thread to disk, creating the directory if needed.
func (t *Thread) Save() error {
	dir := ThreadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, t.ID+".json"), data, 0o644)
}

// CompileContext builds a context string from the thread's history within a byte budget.
func (t *Thread) CompileContext(budget int) (string, bool) {
	if budget <= 0 {
		budget = 32768
	}

	var parts []string
	truncated := false

	if t.Summary != "" {
		parts = append(parts, "Prior context (summarized): "+t.Summary)
	}

	// Build recent turns newest-first, then reverse for chronological order.
	var turnLines []string
	used := 0
	for _, p := range parts {
		used += len(p)
	}

	for i := len(t.Turns) - 1; i >= 0; i-- {
		line := t.Turns[i].Role + ": " + t.Turns[i].Content
		if used+len(line)+1 > budget {
			truncated = true
			break
		}
		turnLines = append(turnLines, line)
		used += len(line) + 1
	}

	// Reverse to chronological order.
	for i, j := 0, len(turnLines)-1; i < j; i, j = i+1, j-1 {
		turnLines[i], turnLines[j] = turnLines[j], turnLines[i]
	}

	if len(turnLines) > 0 {
		parts = append(parts, "Recent conversation:\n"+strings.Join(turnLines, "\n"))
	}

	return strings.Join(parts, "\n\n"), truncated
}

// lastTurnBackend returns the backend that produced the most recent turn, or "".
func (t *Thread) lastTurnBackend() string {
	if len(t.Turns) == 0 {
		return ""
	}
	return t.Turns[len(t.Turns)-1].Backend
}

// prepareThreadPrompt injects thread context into opts if no native session exists.
// Returns the original user prompt (without compiled context) for storage.
func prepareThreadPrompt(thread *Thread, opts *RunOpts) string {
	original := opts.Prompt
	// Only reuse a native session if this backend was the last to contribute.
	// If another backend spoke since, replay canonical context instead.
	if sid, ok := thread.NativeSessions[opts.Backend]; ok && opts.SessionID == "" && thread.lastTurnBackend() == opts.Backend {
		opts.SessionID = sid
		return original
	}
	if len(thread.Turns) > 0 && opts.SessionID == "" {
		ctx, _ := thread.CompileContext(32768)
		opts.Prompt = ctx + "\n\nNew request:\n" + opts.Prompt
	}
	return original
}

// recordTurns appends the user prompt and assistant response to the thread.
func (t *Thread) recordTurns(original string, resp Response) {
	now := time.Now().UTC().Format(time.RFC3339)
	t.Turns = append(t.Turns,
		Turn{Role: "user", Content: original, Backend: resp.Backend, TS: now},
	)
	if resp.Result != "" && resp.Error == "" {
		t.Turns = append(t.Turns,
			Turn{Role: "assistant", Content: resp.Result, Backend: resp.Backend, TS: now},
		)
	}
	if resp.Session != "" {
		t.NativeSessions[resp.Backend] = resp.Session
	}
}

// RunWithThread wraps Run with thread load/save and context injection.
func RunWithThread(backends map[string]Backend, opts RunOpts) Response {
	if opts.ThreadID == "" {
		return Run(backends, opts)
	}

	thread, err := LoadThread(opts.ThreadID)
	if err != nil {
		return Response{Error: err.Error(), Backend: opts.Backend, ThreadID: opts.ThreadID}
	}

	original := prepareThreadPrompt(thread, &opts)
	resp := Run(backends, opts)

	thread.recordTurns(original, resp)
	if err := thread.Save(); err != nil {
		resp.Error = "thread save failed: " + err.Error()
	}
	resp.ThreadID = opts.ThreadID
	return resp
}

// CompactThread summarizes old turns using a backend, keeping the last keepTurns.
func CompactThread(backends map[string]Backend, threadID, backend string) error {
	thread, err := LoadThread(threadID)
	if err != nil {
		return err
	}

	const keepTurns = 4
	if len(thread.Turns) <= keepTurns {
		return nil
	}

	// Build text from old turns to summarize, including prior summary.
	old := thread.Turns[:len(thread.Turns)-keepTurns]
	var lines []string
	if thread.Summary != "" {
		lines = append(lines, "Previous summary: "+thread.Summary)
	}
	for _, t := range old {
		lines = append(lines, t.Role+": "+t.Content)
	}
	text := strings.Join(lines, "\n")

	prompt := "Summarize this conversation concisely, preserving key decisions and context:\n\n" + text
	resp := Run(backends, RunOpts{Backend: backend, Prompt: prompt})
	if resp.Error != "" {
		return fmt.Errorf("compaction failed: %s", resp.Error)
	}

	thread.Summary = resp.Result
	thread.Turns = thread.Turns[len(thread.Turns)-keepTurns:]
	return thread.Save()
}

// ListThreads returns the IDs of all saved threads.
func ListThreads() ([]string, error) {
	entries, err := os.ReadDir(ThreadDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			ids = append(ids, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return ids, nil
}
