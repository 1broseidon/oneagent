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

func (s FilesystemStore) path(id string) string {
	return filepath.Join(s.dir(), id+".json")
}

func threadFilePath(store Store, id string) (string, bool) {
	switch s := store.(type) {
	case FilesystemStore:
		return s.path(id), true
	case *FilesystemStore:
		if s == nil {
			return "", false
		}
		return s.path(id), true
	default:
		return "", false
	}
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

// CompileRecentTurns builds a minimal context string from the most recent turns.
func (t *Thread) CompileRecentTurns(maxTurns, budget int) (string, bool) {
	if maxTurns <= 0 {
		maxTurns = 2
	}
	if budget <= 0 {
		budget = 4096
	}

	var lines []string
	used := len("Conversation:\n")
	truncated := false

	for i := len(t.Turns) - 1; i >= 0; i-- {
		if len(lines) >= maxTurns {
			break
		}
		line := t.Turns[i].Role + ": " + t.Turns[i].Content
		if used+len(line)+1 > budget {
			truncated = true
			break
		}
		lines = append(lines, line)
		used += len(line) + 1
	}

	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	if len(lines) == 0 {
		return "", truncated
	}
	return "Conversation:\n" + strings.Join(lines, "\n"), truncated
}

// lastTurnBackend returns the backend that produced the most recent turn, or "".
func (t *Thread) lastTurnBackend() string {
	if len(t.Turns) == 0 {
		return ""
	}
	return t.Turns[len(t.Turns)-1].Backend
}

// threadFileMetadata reads the thread JSON file and returns total line count
// and the line number where the last turn's JSON object starts.
func threadFileMetadata(path string, turnCount int) (lines int, lastTurnLine int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0
	}
	allLines := strings.Split(string(data), "\n")
	lines = len(allLines)
	if turnCount == 0 {
		return lines, 0
	}
	// Walk backward to find the opening "{" of the last turn object.
	// Each turn is a JSON object inside the "turns" array.
	braceDepth := 0
	turnsFound := 0
	for i := len(allLines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(allLines[i])
		// Count closing braces (enter objects) and opening braces (exit objects).
		if strings.HasPrefix(trimmed, "}") {
			braceDepth++
		}
		if strings.HasPrefix(trimmed, "{") {
			braceDepth--
			if braceDepth == 0 {
				turnsFound++
				// We want the start of the last user+assistant pair (2 turn objects).
				if turnsFound >= 2 {
					lastTurnLine = i + 1 // 1-indexed
					return
				}
			}
		}
	}
	// Fallback: couldn't find enough turns, point near the end.
	if lines > 50 {
		return lines, lines - 50
	}
	return lines, 1
}

// threadTopicSummary extracts a short topic list from user turns.
// It takes the first ~12 words of each user message as a topic hint,
// deduplicates, and returns a compact one-line summary.
func threadTopicSummary(turns []Turn) string {
	seen := make(map[string]bool)
	var topics []string
	for _, t := range turns {
		if t.Role != "user" {
			continue
		}
		msg := strings.TrimSpace(t.Content)
		if msg == "" {
			continue
		}
		words := strings.Fields(msg)
		limit := 12
		if len(words) < limit {
			limit = len(words)
		}
		snippet := strings.Join(words[:limit], " ")
		if len(words) > limit {
			snippet += "..."
		}
		key := strings.ToLower(snippet)
		if seen[key] {
			continue
		}
		seen[key] = true
		topics = append(topics, snippet)
	}
	if len(topics) == 0 {
		return ""
	}
	return strings.Join(topics, " | ")
}

// threadTimeRange returns the timestamps of the first and last turns.
func threadTimeRange(turns []Turn) (first, last string) {
	if len(turns) == 0 {
		return "", ""
	}
	first = turns[0].TS
	last = turns[len(turns)-1].TS
	// Trim to date only for compactness.
	if len(first) >= 10 {
		first = first[:10]
	}
	if len(last) >= 10 {
		last = last[:10]
	}
	return
}

// prepareThreadPrompt injects portable thread handoff instructions into opts if no native session exists.
func prepareThreadPrompt(thread *Thread, store Store, opts *RunOpts) {
	// Only reuse a native session if this backend was the last to contribute.
	// If another backend spoke since, point the backend at the canonical thread file.
	if sid, ok := thread.NativeSessions[opts.Backend]; ok && opts.SessionID == "" && thread.lastTurnBackend() == opts.Backend {
		opts.SessionID = sid
		return
	}
	if len(thread.Turns) > 0 && opts.SessionID == "" {
		if path, ok := threadFilePath(store, thread.ID); ok {
			totalLines, lastTurnLine := threadFileMetadata(path, len(thread.Turns))
			firstDate, lastDate := threadTimeRange(thread.Turns)
			topics := threadTopicSummary(thread.Turns)

			var meta strings.Builder
			meta.WriteString("You are continuing conversation thread \"" + thread.ID + "\".\n")
			meta.WriteString("Thread file: " + path + "\n")
			if totalLines > 0 {
				meta.WriteString(fmt.Sprintf("Lines: %d\n", totalLines))
			}
			if lastTurnLine > 0 {
				meta.WriteString(fmt.Sprintf("Last turn starts at line: %d\n", lastTurnLine))
			}
			meta.WriteString(fmt.Sprintf("Turns: %d", len(thread.Turns)))
			if firstDate != "" && lastDate != "" {
				if firstDate == lastDate {
					meta.WriteString(fmt.Sprintf(" (%s)", firstDate))
				} else {
					meta.WriteString(fmt.Sprintf(" (%s to %s)", firstDate, lastDate))
				}
			}
			meta.WriteString("\n")
			if topics != "" {
				meta.WriteString("Topics: " + topics + "\n")
			}
			meta.WriteString("Read the thread JSON file and continue from the last turn.\n\n")
			meta.WriteString("Current user message:\n" + opts.Prompt)

			opts.Prompt = meta.String()
			return
		}
		ctx, _ := thread.CompileContext(32768)
		if strings.TrimSpace(ctx) != "" {
			opts.Prompt = ctx + "\n\nCurrent user message:\n" + opts.Prompt
		}
	}
}

// recordTurns appends the user prompt and assistant response to the thread.
// It deduplicates against the last turn to prevent concurrent writers from
// recording the same content twice.
func (t *Thread) recordTurns(original string, resp Response, source string) {
	now := time.Now().UTC().Format(time.RFC3339)
	if !t.lastTurnMatches("user", original) {
		t.Turns = append(t.Turns,
			Turn{Role: "user", Content: original, Backend: resp.Backend, Source: source, TS: now},
		)
	}
	if resp.Result != "" && resp.Error == "" {
		if !t.lastTurnMatches("assistant", resp.Result) {
			t.Turns = append(t.Turns,
				Turn{Role: "assistant", Content: resp.Result, Backend: resp.Backend, Source: source, TS: now},
			)
		}
	}
	if resp.Session != "" {
		t.NativeSessions[resp.Backend] = resp.Session
	}
}

// lastTurnMatches returns true if the most recent turn with the given role
// has identical content, indicating a duplicate from concurrent recording.
func (t *Thread) lastTurnMatches(role, content string) bool {
	for i := len(t.Turns) - 1; i >= 0; i-- {
		if t.Turns[i].Role == role {
			return t.Turns[i].Content == content
		}
		// Only check the most recent turn of each role — stop if we hit
		// the opposite role first (normal alternating pattern).
		if t.Turns[i].Role != role {
			return false
		}
	}
	return false
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
