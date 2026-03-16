package oneagent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// staticBackend returns a backend that emits a fixed result and session.
func staticBackend(result, session string) Backend {
	return Backend{
		Cmd:     []string{"sh", "-c", `printf '{"result":"` + result + `","session":"` + session + `"}'`},
		Format:  "json",
		Result:  "result",
		Session: "session",
	}
}

// resumeDetectBackend returns "FRESH" on first call, "RESUMED" when resume_cmd is used.
func resumeDetectBackend(session string) Backend {
	return Backend{
		Cmd:       []string{"sh", "-c", `printf '{"result":"FRESH","session":"` + session + `"}'`},
		ResumeCmd: []string{"sh", "-c", `printf '{"result":"RESUMED","session":"` + session + `"}'`},
		Format:    "json",
		Result:    "result",
		Session:   "session",
	}
}

func setupThreadHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
}

func TestNewThreadCreatesFileAndRecordsTurns(t *testing.T) {
	setupThreadHome(t)

	backends := map[string]Backend{"a": staticBackend("ok", "sa")}
	resp := RunWithThread(backends, RunOpts{Backend: "a", Prompt: "first", ThreadID: "t1"})

	if resp.ThreadID != "t1" {
		t.Fatalf("thread_id = %q, want t1", resp.ThreadID)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}

	// Verify file exists.
	path := filepath.Join(ThreadDir(), "t1.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("thread file not created: %v", err)
	}

	thread, err := LoadThread("t1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(thread.Turns) != 2 {
		t.Fatalf("turns = %d, want 2", len(thread.Turns))
	}
	if thread.Turns[0].Role != "user" || thread.Turns[0].Content != "first" {
		t.Fatalf("turn[0] = %+v, want user/first", thread.Turns[0])
	}
	if thread.Turns[1].Role != "assistant" {
		t.Fatalf("turn[1].role = %q, want assistant", thread.Turns[1].Role)
	}
}

func TestSameBackendReusesNativeSession(t *testing.T) {
	setupThreadHome(t)

	backends := map[string]Backend{"a": resumeDetectBackend("sa")}

	// First call — no session yet.
	resp1 := RunWithThread(backends, RunOpts{Backend: "a", Prompt: "one", ThreadID: "t2"})
	if resp1.Error != "" {
		t.Fatalf("call 1 error: %s", resp1.Error)
	}

	// Second call — same backend, should resume via native session.
	resp2 := RunWithThread(backends, RunOpts{Backend: "a", Prompt: "two", ThreadID: "t2"})
	if resp2.Error != "" {
		t.Fatalf("call 2 error: %s", resp2.Error)
	}
	if resp2.Result != "RESUMED" {
		t.Fatalf("second call should use resume_cmd, got result = %q", resp2.Result)
	}

	thread, _ := LoadThread("t2")
	if thread.NativeSessions["a"] != "sa" {
		t.Fatalf("native session not saved, got %v", thread.NativeSessions)
	}
}

func TestRunWithThreadStreamEmitsThreadEventsAndPersistsSession(t *testing.T) {
	setupThreadHome(t)

	events := `{"type":"session","sid":"sa"}
{"type":"delta","data":{"text":"hello"}}
`
	b := fakeJSONL(events)
	b.ResultAppend = true
	b.SessionWhen = "type=session"
	backends := map[string]Backend{"a": b}

	var got []StreamEvent
	resp := RunWithThreadStream(backends, RunOpts{Backend: "a", Prompt: "one", ThreadID: "ts"}, func(event StreamEvent) {
		got = append(got, event)
	})

	if resp.ThreadID != "ts" {
		t.Fatalf("thread_id = %q, want ts", resp.ThreadID)
	}
	if len(got) != 3 {
		t.Fatalf("events = %+v, want 3 events", got)
	}
	if got[0].Type != "session" || got[0].ThreadID != "ts" {
		t.Fatalf("first event = %+v, want session with thread id", got[0])
	}
	if got[2].Type != "done" || got[2].ThreadID != "ts" {
		t.Fatalf("final event = %+v, want done with thread id", got[2])
	}

	thread, err := LoadThread("ts")
	if err != nil {
		t.Fatalf("load thread: %v", err)
	}
	if thread.NativeSessions["a"] != "sa" {
		t.Fatalf("native session = %q, want sa", thread.NativeSessions["a"])
	}
	if len(thread.Turns) != 2 {
		t.Fatalf("turns = %d, want 2", len(thread.Turns))
	}
}

func TestCrossBackendReplaysCanonicalContext(t *testing.T) {
	setupThreadHome(t)

	backends := map[string]Backend{
		"a": resumeDetectBackend("sa"),
		"b": resumeDetectBackend("sb"),
	}

	// A: first turn.
	RunWithThread(backends, RunOpts{Backend: "a", Prompt: "step one", ThreadID: "t3"})
	// B: second turn — different backend.
	RunWithThread(backends, RunOpts{Backend: "b", Prompt: "step two", ThreadID: "t3"})
	// A: third turn — should NOT resume stale native session.
	resp := RunWithThread(backends, RunOpts{Backend: "a", Prompt: "step three", ThreadID: "t3"})

	if resp.Error != "" {
		t.Fatalf("A->B->A error: %s", resp.Error)
	}
	// If native session were reused, result would be "RESUMED".
	// Canonical replay uses Cmd (not ResumeCmd), so result should be "FRESH".
	if resp.Result == "RESUMED" {
		t.Fatal("A->B->A should replay canonical context, not reuse stale native session")
	}
}

func TestFailedRunDoesNotRecordAssistantTurn(t *testing.T) {
	setupThreadHome(t)

	backends := map[string]Backend{
		"bad": {
			Cmd:    []string{"sh", "-c", "exit 1"},
			Format: "json",
			Result: "result",
		},
	}
	RunWithThread(backends, RunOpts{Backend: "bad", Prompt: "boom", ThreadID: "t4"})

	thread, _ := LoadThread("t4")
	for _, turn := range thread.Turns {
		if turn.Role == "assistant" {
			t.Fatal("failed run should not record assistant turn")
		}
	}
}

func TestNativeSessionsTrackedPerBackend(t *testing.T) {
	setupThreadHome(t)

	backends := map[string]Backend{
		"a": staticBackend("ok-a", "sess-a"),
		"b": staticBackend("ok-b", "sess-b"),
	}
	RunWithThread(backends, RunOpts{Backend: "a", Prompt: "one", ThreadID: "t5"})
	RunWithThread(backends, RunOpts{Backend: "b", Prompt: "two", ThreadID: "t5"})

	thread, _ := LoadThread("t5")
	if thread.NativeSessions["a"] != "sess-a" {
		t.Fatalf("native session a = %q, want sess-a", thread.NativeSessions["a"])
	}
	if thread.NativeSessions["b"] != "sess-b" {
		t.Fatalf("native session b = %q, want sess-b", thread.NativeSessions["b"])
	}
}

func TestCompactionNoOpForShortThreads(t *testing.T) {
	setupThreadHome(t)

	thread := &Thread{
		ID:             "short",
		Turns:          []Turn{{Role: "user", Content: "hi"}, {Role: "assistant", Content: "hey"}},
		NativeSessions: map[string]string{},
	}
	thread.Save()

	backends := map[string]Backend{"s": staticBackend("ok", "")}
	err := CompactThread(backends, "short", "s")
	if err != nil {
		t.Fatalf("compaction should be no-op, got: %v", err)
	}

	after, _ := LoadThread("short")
	if len(after.Turns) != 2 {
		t.Fatalf("turns changed on no-op compaction: %d", len(after.Turns))
	}
	if after.Summary != "" {
		t.Fatalf("summary set on no-op compaction: %q", after.Summary)
	}
}

func TestCompactionKeepsLastNTurns(t *testing.T) {
	setupThreadHome(t)

	turns := make([]Turn, 8)
	for i := range turns {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		turns[i] = Turn{Role: role, Content: strings.Repeat("x", i+1)}
	}

	thread := &Thread{ID: "keep", Turns: turns, NativeSessions: map[string]string{}}
	thread.Save()

	// Summarizer backend returns a fixed summary.
	backends := map[string]Backend{
		"s": fakeJSON("the-summary", ""),
	}
	if err := CompactThread(backends, "keep", "s"); err != nil {
		t.Fatalf("compaction failed: %v", err)
	}

	after, _ := LoadThread("keep")
	if after.Summary != "the-summary" {
		t.Fatalf("summary = %q, want the-summary", after.Summary)
	}
	if len(after.Turns) != 4 {
		t.Fatalf("turns = %d, want 4 (last N kept)", len(after.Turns))
	}
	// The kept turns should be the last 4 from the original.
	if after.Turns[0].Content != "xxxxx" {
		t.Fatalf("first kept turn = %q, want 5 x's", after.Turns[0].Content)
	}
}

func TestRepeatedCompactionPreservesSummary(t *testing.T) {
	setupThreadHome(t)

	// Summarizer echoes its entire prompt as the result.
	backends := map[string]Backend{
		"s": {
			Cmd:     []string{"sh", "-c", `printf '{"result":"%s","session":""}' "$0"`},
			Format:  "json",
			Result:  "result",
			Session: "session",
		},
	}

	// 8 turns, compact once.
	turns := make([]Turn, 8)
	for i := range turns {
		turns[i] = Turn{Role: "user", Content: "msg"}
	}
	thread := &Thread{ID: "rep", Turns: turns, NativeSessions: map[string]string{}}
	thread.Save()

	CompactThread(backends, "rep", "s")
	after1, _ := LoadThread("rep")
	firstSummary := after1.Summary

	// Add more turns and compact again.
	for i := 0; i < 6; i++ {
		after1.Turns = append(after1.Turns, Turn{Role: "user", Content: "extra"})
	}
	after1.Save()

	CompactThread(backends, "rep", "s")
	after2, _ := LoadThread("rep")

	// The second compaction's input should have included the first summary.
	// Since the summarizer echoes its prompt, the result should contain the first summary.
	if !strings.Contains(after2.Summary, firstSummary) {
		t.Fatalf("repeated compaction lost prior summary.\nfirst:  %q\nsecond: %q", firstSummary, after2.Summary)
	}
}

func TestCompileContextRespectsBudget(t *testing.T) {
	thread := &Thread{
		Turns: []Turn{
			{Role: "user", Content: strings.Repeat("a", 100)},
			{Role: "assistant", Content: strings.Repeat("b", 100)},
			{Role: "user", Content: strings.Repeat("c", 100)},
		},
	}
	ctx, truncated := thread.CompileContext(150)
	if !truncated {
		t.Fatal("expected truncation with small budget")
	}
	// Should include the newest turn(s) that fit.
	if !strings.Contains(ctx, strings.Repeat("c", 100)) {
		t.Fatal("newest turn should be included")
	}
}

func TestCompileContextIncludesSummary(t *testing.T) {
	thread := &Thread{
		Summary: "prior decisions here",
		Turns:   []Turn{{Role: "user", Content: "latest"}},
	}
	ctx, _ := thread.CompileContext(0)
	if !strings.Contains(ctx, "Prior context (summarized): prior decisions here") {
		t.Fatalf("context should include summary, got %q", ctx)
	}
	if !strings.Contains(ctx, "user: latest") {
		t.Fatalf("context should include turns, got %q", ctx)
	}
}

func TestListThreads(t *testing.T) {
	setupThreadHome(t)

	for _, id := range []string{"alpha", "beta"} {
		th := &Thread{ID: id, NativeSessions: map[string]string{}}
		th.Save()
	}

	ids, err := ListThreads()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("listed %d threads, want 2", len(ids))
	}

	found := map[string]bool{}
	for _, id := range ids {
		found[id] = true
	}
	if !found["alpha"] || !found["beta"] {
		t.Fatalf("missing thread IDs, got %v", ids)
	}
}

func TestListThreadsEmptyDir(t *testing.T) {
	setupThreadHome(t)
	ids, err := ListThreads()
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected empty list, got %v", ids)
	}
}

func TestLoadThreadMissingFileReturnsEmpty(t *testing.T) {
	setupThreadHome(t)
	thread, err := LoadThread("nonexistent")
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if thread.ID != "nonexistent" || len(thread.Turns) != 0 {
		t.Fatalf("expected empty thread, got %+v", thread)
	}
}

func TestLoadThreadCorruptFile(t *testing.T) {
	setupThreadHome(t)
	dir := ThreadDir()
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{not json"), 0o644)

	_, err := LoadThread("bad")
	if err == nil {
		t.Fatal("expected error for corrupt thread file")
	}
}

func TestRunWithThreadEmptyIDCallsRunDirectly(t *testing.T) {
	backends := map[string]Backend{"a": fakeJSON("direct", "")}
	resp := RunWithThread(backends, RunOpts{Backend: "a", Prompt: "hi"})
	if resp.Result != "direct" {
		t.Fatalf("result = %q, want direct", resp.Result)
	}
	if resp.ThreadID != "" {
		t.Fatalf("thread_id should be empty for direct call, got %q", resp.ThreadID)
	}
}

func TestThreadJSONRoundTrip(t *testing.T) {
	thread := &Thread{
		ID:      "rt",
		Summary: "sum",
		Turns: []Turn{
			{Role: "user", Content: "hi", Backend: "a", TS: "2025-01-01T00:00:00Z"},
		},
		NativeSessions: map[string]string{"a": "s1"},
	}
	data, err := json.Marshal(thread)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Thread
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != "rt" || got.Summary != "sum" || len(got.Turns) != 1 || got.NativeSessions["a"] != "s1" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}
