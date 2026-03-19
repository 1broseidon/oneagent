package oneagent

import (
	"encoding/json"
	"fmt"
	"io"
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
	Source  string `json:"source,omitempty"`
	TS      string `json:"ts"`
}

// Thread is a portable conversation that can span multiple backends.
type Thread struct {
	ID             string            `json:"id"`
	Summary        string            `json:"summary,omitempty"`
	Turns          []Turn            `json:"turns"`
	NativeSessions map[string]string `json:"native_sessions,omitempty"`
}

// Store persists portable thread state for a Client.
type Store interface {
	LoadThread(id string) (*Thread, error)
	SaveThread(thread *Thread) error
	ListThreads() ([]string, error)
}

// FilesystemStore stores thread JSON files in a directory on disk.
type FilesystemStore struct {
	Dir string
}

// ThreadDir returns the directory for thread storage.
func ThreadDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "oneagent", "threads")
}

func (s FilesystemStore) dir() string {
	if s.Dir != "" {
		return s.Dir
	}
	return ThreadDir()
}

func validateThreadID(id string) error {
	switch {
	case id == ".", id == "..":
		return fmt.Errorf("invalid thread id %q", id)
	case strings.ContainsAny(id, "/\\"):
		return fmt.Errorf("invalid thread id %q", id)
	default:
		return nil
	}
}

// LoadThread reads a thread from disk. A missing file returns an empty thread.
func LoadThread(id string) (*Thread, error) {
	return FilesystemStore{}.LoadThread(id)
}

// LoadThread reads a thread from the configured store. A missing thread returns an empty thread.
func (c Client) LoadThread(id string) (*Thread, error) {
	return c.threadStore().LoadThread(id)
}

// LoadThread reads a thread from the filesystem store. A missing file returns an empty thread.
func (s FilesystemStore) LoadThread(id string) (*Thread, error) {
	if err := validateThreadID(id); err != nil {
		return nil, err
	}
	path := filepath.Join(s.dir(), id+".json")
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &Thread{ID: id, NativeSessions: map[string]string{}}, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	if err := flockShared(f.Fd()); err != nil {
		return nil, err
	}
	defer func() {
		_ = flockUnlock(f.Fd())
	}()
	data, err := io.ReadAll(f)
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
	return FilesystemStore{}.SaveThread(t)
}

// SaveThread writes the thread to the configured store.
func (c Client) SaveThread(t *Thread) error {
	return c.threadStore().SaveThread(t)
}

// SaveThread writes the thread to disk, creating the directory if needed.
func (s FilesystemStore) SaveThread(t *Thread) error {
	data, path, err := s.prepareThreadSave(t)
	if err != nil {
		return err
	}
	return saveThreadFile(path, data)
}

func (s FilesystemStore) prepareThreadSave(t *Thread) ([]byte, string, error) {
	if err := validateThreadID(t.ID); err != nil {
		return nil, "", err
	}
	dir := s.dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return data, filepath.Join(dir, t.ID+".json"), nil
}

func saveThreadFile(path string, data []byte) error {
	f, err := openLockedThreadFile(path)
	if err != nil {
		return err
	}
	defer closeLockedThreadFile(f)
	return writeThreadTempFile(path, data)
}

func openLockedThreadFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	if err := flockExcl(f.Fd()); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func closeLockedThreadFile(f *os.File) {
	_ = flockUnlock(f.Fd())
	_ = f.Close()
}

func writeThreadTempFile(path string, data []byte) error {
	tmpPath := path + ".tmp"
	tmp, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if err := writeAndCloseTempFile(tmp, data); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func writeAndCloseTempFile(tmp *os.File, data []byte) error {
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	return tmp.Close()
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
func prepareThreadPrompt(thread *Thread, opts *RunOpts) {
	// Only reuse a native session if this backend was the last to contribute.
	// If another backend spoke since, replay canonical context instead.
	if sid, ok := thread.NativeSessions[opts.Backend]; ok && opts.SessionID == "" && thread.lastTurnBackend() == opts.Backend {
		opts.SessionID = sid
		return
	}
	if len(thread.Turns) > 0 && opts.SessionID == "" {
		ctx, _ := thread.CompileContext(32768)
		opts.Prompt = ctx + "\n\nNew request:\n" + opts.Prompt
	}
}

// recordTurns appends the user prompt and assistant response to the thread.
func (t *Thread) recordTurns(original string, resp Response, source string) {
	now := time.Now().UTC().Format(time.RFC3339)
	t.Turns = append(t.Turns,
		Turn{Role: "user", Content: original, Backend: resp.Backend, Source: source, TS: now},
	)
	if resp.Result != "" && resp.Error == "" {
		t.Turns = append(t.Turns,
			Turn{Role: "assistant", Content: resp.Result, Backend: resp.Backend, Source: source, TS: now},
		)
	}
	if resp.Session != "" {
		t.NativeSessions[resp.Backend] = resp.Session
	}
}

// RunWithThread wraps Run with thread load/save and context injection.
func RunWithThread(backends map[string]Backend, opts RunOpts) Response {
	return Client{Backends: backends}.RunWithThread(opts)
}

// RunWithThreadStream wraps RunStream with thread load/save and context injection.
func RunWithThreadStream(backends map[string]Backend, opts RunOpts, emit func(StreamEvent)) Response {
	return Client{Backends: backends}.RunWithThreadStream(opts, emit)
}

// RunWithThread wraps Run with thread load/save and context injection.
// Threading is handled by invoke() when ThreadID is set.
func (c Client) RunWithThread(opts RunOpts) Response {
	return c.invoke(opts, nil)
}

// RunWithThreadStream wraps RunStream with thread load/save and context injection.
// Threading is handled by invoke() when ThreadID is set.
func (c Client) RunWithThreadStream(opts RunOpts, emit func(StreamEvent)) Response {
	return c.invoke(opts, emit)
}

// CompactThread summarizes old turns using a backend, keeping the last keepTurns.
func CompactThread(backends map[string]Backend, threadID, backend string) error {
	return Client{Backends: backends}.CompactThread(threadID, backend)
}

// CompactThread summarizes old turns using a backend, keeping the last keepTurns.
func (c Client) CompactThread(threadID, backend string) error {
	thread, err := c.threadStore().LoadThread(threadID)
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
	resp := c.runDirect(RunOpts{Backend: backend, Prompt: prompt})
	if resp.Error != "" {
		return fmt.Errorf("compaction failed: %s", resp.Error)
	}

	thread.Summary = resp.Result
	thread.Turns = thread.Turns[len(thread.Turns)-keepTurns:]
	return c.threadStore().SaveThread(thread)
}

// ListThreads returns the IDs of all saved threads.
func ListThreads() ([]string, error) {
	return FilesystemStore{}.ListThreads()
}

// ListThreads returns the IDs of all saved threads from the configured store.
func (c Client) ListThreads() ([]string, error) {
	return c.threadStore().ListThreads()
}

// ListThreads returns the IDs of all saved threads from the filesystem store.
func (s FilesystemStore) ListThreads() ([]string, error) {
	entries, err := os.ReadDir(s.dir())
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

func (c Client) threadStore() Store {
	if c.Store != nil {
		return c.Store
	}
	return FilesystemStore{}
}
