package oneagent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
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

func echoPromptBackend() Backend {
	return Backend{
		Cmd:         []string{"sh", "-c", `python3 -c 'import json,sys; print(json.dumps({"result": sys.stdin.read(), "session": "sess"}))'`},
		Format:      "json",
		Result:      "result",
		Session:     "session",
		PromptStdin: true,
	}
}

func setupThreadHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
}

type memoryStore struct {
	threads map[string]*Thread
}

func newMemoryStore() *memoryStore {
	return &memoryStore{threads: map[string]*Thread{}}
}

func (s *memoryStore) LoadThread(id string) (*Thread, error) {
	if thread, ok := s.threads[id]; ok {
		data, _ := json.Marshal(thread)
		var clone Thread
		json.Unmarshal(data, &clone)
		if clone.NativeSessions == nil {
			clone.NativeSessions = map[string]string{}
		}
		return &clone, nil
	}
	return &Thread{ID: id, NativeSessions: map[string]string{}}, nil
}

func (s *memoryStore) SaveThread(thread *Thread) error {
	data, _ := json.Marshal(thread)
	var clone Thread
	json.Unmarshal(data, &clone)
	s.threads[thread.ID] = &clone
	return nil
}

func (s *memoryStore) ListThreads() ([]string, error) {
	ids := make([]string, 0, len(s.threads))
	for id := range s.threads {
		ids = append(ids, id)
	}
	return ids, nil
}

func TestNewThreadCreatesFileAndRecordsTurns(t *testing.T) {
	setupThreadHome(t)

	backends := map[string]Backend{"a": staticBackend("ok", "sa")}
	resp := RunWithThread(backends, RunOpts{Backend: "a", Prompt: "first", ThreadID: "t1", Source: "telegram"})

	if resp.ThreadID != "t1" {
		t.Fatalf("thread_id = %q, want t1", resp.ThreadID)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}

	assertThreadFileExists(t, "t1")
	thread, err := LoadThread("t1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(thread.Turns) != 2 {
		t.Fatalf("turns = %d, want 2", len(thread.Turns))
	}
	assertTurn(t, thread.Turns[0], "user", "first", "telegram")
	assertTurn(t, thread.Turns[1], "assistant", "ok", "telegram")
}

func TestFilesystemStoreUsesCustomDir(t *testing.T) {
	store := FilesystemStore{Dir: t.TempDir()}
	client := Client{
		Backends: map[string]Backend{"a": staticBackend("ok", "sa")},
		Store:    store,
	}

	resp := client.RunWithThread(RunOpts{Backend: "a", Prompt: "first", ThreadID: "custom"})
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}

	path := filepath.Join(store.Dir, "custom.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("thread file not created in custom dir: %v", err)
	}
}

func TestThreadIDTraversal(t *testing.T) {
	store := FilesystemStore{Dir: t.TempDir()}

	for _, id := range []string{"../escape", "/abs", `a\b`, ".", ".."} {
		t.Run(id, func(t *testing.T) {
			if _, err := store.LoadThread(id); err == nil {
				t.Fatalf("LoadThread(%q) should reject invalid thread IDs", id)
			}
			if err := store.SaveThread(&Thread{ID: id, NativeSessions: map[string]string{}}); err == nil {
				t.Fatalf("SaveThread(%q) should reject invalid thread IDs", id)
			}
		})
	}
}

func TestClientUsesCustomInMemoryStore(t *testing.T) {
	store := newMemoryStore()
	client := Client{
		Backends: map[string]Backend{"a": staticBackend("ok", "sa")},
		Store:    store,
	}

	resp := client.RunWithThread(RunOpts{Backend: "a", Prompt: "first", ThreadID: "mem"})
	if resp.ThreadID != "mem" {
		t.Fatalf("thread_id = %q, want mem", resp.ThreadID)
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}

	thread, err := client.LoadThread("mem")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(thread.Turns) != 2 {
		t.Fatalf("turns = %d, want 2", len(thread.Turns))
	}

	ids, err := client.ListThreads()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(ids) != 1 || ids[0] != "mem" {
		t.Fatalf("ids = %v, want [mem]", ids)
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
	if len(got) != 4 {
		t.Fatalf("events = %+v, want 4 events", got)
	}
	if got[0].Type != "start" || got[0].ThreadID != "ts" {
		t.Fatalf("first event = %+v, want start with thread id", got[0])
	}
	if got[1].Type != "session" || got[1].ThreadID != "ts" {
		t.Fatalf("second event = %+v, want session with thread id", got[1])
	}
	if got[3].Type != "done" || got[3].ThreadID != "ts" {
		t.Fatalf("final event = %+v, want done with thread id", got[3])
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

func TestPostRunHookReceivesResultAndEnv(t *testing.T) {
	setupThreadHome(t)

	home := os.Getenv("HOME")
	resultPath := filepath.Join(home, "hook result.txt")
	envPath := filepath.Join(home, "hook env.txt")
	scriptPath := filepath.Join(home, "hook.sh")
	script := "#!/bin/sh\n" +
		"cat > \"$1\"\n" +
		"printf 'OA_THREAD_ID=%s\nOA_BACKEND=%s\nOA_SESSION=%s\nOA_SOURCE=%s\nOA_EXIT=%s\nOA_ERROR=%s\n' " +
		"\"$OA_THREAD_ID\" \"$OA_BACKEND\" \"$OA_SESSION\" \"$OA_SOURCE\" \"$OA_EXIT\" \"$OA_ERROR\" > \"$2\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write hook script: %v", err)
	}

	backends := map[string]Backend{"a": staticBackend("ok", "sa")}
	resp := RunWithThread(backends, RunOpts{
		Backend:    "a",
		Prompt:     "first",
		ThreadID:   "t-hook",
		Source:     "telegram",
		PostRunCmd: fmt.Sprintf("%s '%s' '%s'", scriptPath, resultPath, envPath),
	})

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}

	result, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("read hook result: %v", err)
	}
	if string(result) != "ok" {
		t.Fatalf("hook stdin = %q, want ok", string(result))
	}

	env, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read hook env: %v", err)
	}
	gotEnv := string(env)
	for _, want := range []string{
		"OA_THREAD_ID=t-hook",
		"OA_BACKEND=a",
		"OA_SESSION=sa",
		"OA_SOURCE=telegram",
		"OA_EXIT=0",
	} {
		if !strings.Contains(gotEnv, want) {
			t.Fatalf("hook env missing %q:\n%s", want, gotEnv)
		}
	}
}

func TestPostRunHookIsBestEffort(t *testing.T) {
	setupThreadHome(t)

	var logs bytes.Buffer
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	oldPrefix := log.Prefix()
	log.SetOutput(&logs)
	log.SetFlags(0)
	log.SetPrefix("")
	defer func() {
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
		log.SetPrefix(oldPrefix)
	}()

	backends := map[string]Backend{"a": staticBackend("ok", "sa")}
	resp := RunWithThread(backends, RunOpts{
		Backend:    "a",
		Prompt:     "first",
		ThreadID:   "t-hook-fail",
		PostRunCmd: "sh -c 'echo hook failed >&2; exit 17'",
	})

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if !strings.Contains(logs.String(), "post-run hook failed") {
		t.Fatalf("expected hook failure to be logged, got %q", logs.String())
	}
}

func TestPreRunHookAborts(t *testing.T) {
	setupThreadHome(t)

	home := os.Getenv("HOME")
	markerPath := filepath.Join(home, "hook marker.txt")
	scriptPath := filepath.Join(home, "marker.sh")
	script := "#!/bin/sh\ncat > \"$1\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write marker script: %v", err)
	}

	backends := map[string]Backend{"a": staticBackend("ok", "sa")}
	resp := Run(backends, RunOpts{
		Backend:    "a",
		Prompt:     "hi",
		PreRunCmd:  "sh -c 'exit 1'",
		PostRunCmd: fmt.Sprintf("%s '%s'", scriptPath, markerPath),
	})

	if resp.Error == "" {
		t.Fatal("expected pre-run abort error")
	}
	if !strings.Contains(resp.Error, "cli pre_run hook") {
		t.Fatalf("error should mention cli pre_run hook, got %q", resp.Error)
	}
	// Post-run hook should NOT have run because pre-run aborted
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("post-run hook should not have run after pre-run abort, stat err = %v", err)
	}
}

func TestPreRunCallbackCanModifyOpts(t *testing.T) {
	backends := map[string]Backend{"a": staticBackend("ok", "sa")}

	var captured string
	resp := Run(backends, RunOpts{
		Backend: "a",
		Prompt:  "original",
		PreRun: func(opts *RunOpts) error {
			captured = opts.Prompt
			return nil
		},
	})

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if captured != "original" {
		t.Fatalf("PreRun callback did not receive opts, got prompt=%q", captured)
	}
}

func TestPreRunCallbackAborts(t *testing.T) {
	backends := map[string]Backend{"a": staticBackend("ok", "sa")}

	resp := Run(backends, RunOpts{
		Backend: "a",
		Prompt:  "hi",
		PreRun: func(opts *RunOpts) error {
			return fmt.Errorf("denied")
		},
	})

	if resp.Error == "" {
		t.Fatal("expected pre-run callback abort")
	}
	if !strings.Contains(resp.Error, "denied") {
		t.Fatalf("error should contain denial reason, got %q", resp.Error)
	}
}

func TestPostRunCallbackReceivesContext(t *testing.T) {
	backends := map[string]Backend{"a": staticBackend("ok", "sa")}

	var got *HookContext
	resp := Run(backends, RunOpts{
		Backend: "a",
		Prompt:  "hi",
		PostRun: func(ctx *HookContext) {
			got = ctx
		},
	})

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if got == nil {
		t.Fatal("PostRun callback not called")
	}
	if got.Response.Result != "ok" {
		t.Fatalf("PostRun result = %q, want ok", got.Response.Result)
	}
	if got.Opts.Backend != "a" {
		t.Fatalf("PostRun backend = %q, want a", got.Opts.Backend)
	}
}

func TestPostRunOriginalPrompt(t *testing.T) {
	store := newMemoryStore()
	store.threads["threaded"] = &Thread{
		ID: "threaded",
		Turns: []Turn{
			{Role: "user", Content: "earlier", Backend: "b"},
			{Role: "assistant", Content: "done", Backend: "b"},
		},
		NativeSessions: map[string]string{},
	}
	client := Client{
		Backends: map[string]Backend{"a": staticBackend("ok", "sa")},
		Store:    store,
	}

	var gotPrompt string
	resp := client.RunWithThread(RunOpts{
		Backend:  "a",
		Prompt:   "current request",
		ThreadID: "threaded",
		PostRun: func(ctx *HookContext) {
			gotPrompt = ctx.Opts.Prompt
		},
	})

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if gotPrompt != "current request" {
		t.Fatalf("PostRun prompt = %q, want original prompt", gotPrompt)
	}
}

func TestConfigAndCLIHooksStack(t *testing.T) {
	setupThreadHome(t)

	home := os.Getenv("HOME")
	configMarker := filepath.Join(home, "config_hook.txt")
	cliMarker := filepath.Join(home, "cli_hook.txt")

	configScript := filepath.Join(home, "config_hook.sh")
	if err := os.WriteFile(configScript, []byte("#!/bin/sh\necho config > \"$1\"\n"), 0o755); err != nil {
		t.Fatalf("write config hook: %v", err)
	}
	cliScript := filepath.Join(home, "cli_hook.sh")
	if err := os.WriteFile(cliScript, []byte("#!/bin/sh\necho cli > \"$1\"\n"), 0o755); err != nil {
		t.Fatalf("write cli hook: %v", err)
	}

	b := staticBackend("ok", "sa")
	b.PostRunCmd = fmt.Sprintf("%s '%s'", configScript, configMarker)
	backends := map[string]Backend{"a": b}

	resp := Run(backends, RunOpts{
		Backend:    "a",
		Prompt:     "hi",
		PostRunCmd: fmt.Sprintf("%s '%s'", cliScript, cliMarker),
	})

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}

	// Both hooks should have run
	if _, err := os.Stat(configMarker); err != nil {
		t.Fatalf("config post-run hook did not run: %v", err)
	}
	if _, err := os.Stat(cliMarker); err != nil {
		t.Fatalf("cli post-run hook did not run: %v", err)
	}
}

func TestCompactThreadDoesNotTriggerHooks(t *testing.T) {
	setupThreadHome(t)

	home := os.Getenv("HOME")
	markerPath := filepath.Join(home, "compact_hook.txt")
	hookScript := filepath.Join(home, "compact_hook.sh")
	if err := os.WriteFile(hookScript, []byte("#!/bin/sh\necho fired > \"$1\"\n"), 0o755); err != nil {
		t.Fatalf("write hook: %v", err)
	}

	b := fakeJSON("the-summary", "")
	b.PostRunCmd = fmt.Sprintf("%s '%s'", hookScript, markerPath)
	backends := map[string]Backend{"s": b}

	turns := make([]Turn, 8)
	for i := range turns {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		turns[i] = Turn{Role: role, Content: strings.Repeat("x", i+1)}
	}

	thread := &Thread{ID: "compact-hooks", Turns: turns, NativeSessions: map[string]string{}}
	thread.Save()

	if err := CompactThread(backends, "compact-hooks", "s"); err != nil {
		t.Fatalf("compaction failed: %v", err)
	}

	// The hook should NOT have fired since CompactThread uses runDirect
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("hooks should not fire during CompactThread, stat err = %v", err)
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
	// Portable fallback uses Cmd (not ResumeCmd), so result should be "FRESH".
	if resp.Result == "RESUMED" {
		t.Fatal("A->B->A should use portable fallback, not reuse stale native session")
	}
}

func TestCrossBackendPromptUsesThreadFileHandoff(t *testing.T) {
	setupThreadHome(t)

	backends := map[string]Backend{
		"a": staticBackend("ok-a", "sess-a"),
		"b": echoPromptBackend(),
	}

	RunWithThread(backends, RunOpts{Backend: "a", Prompt: "step one", ThreadID: "handoff"})
	resp := RunWithThread(backends, RunOpts{Backend: "b", Prompt: "step two", ThreadID: "handoff"})
	if resp.Error != "" {
		t.Fatalf("cross-backend handoff error: %s", resp.Error)
	}
	wantPath := filepath.Join(ThreadDir(), "handoff.json")
	for _, want := range []string{
		`You are continuing conversation thread "handoff".`,
		"Thread file: " + wantPath,
		"Read the thread JSON file and continue from the last turn.",
		"Current user message:\nstep two",
	} {
		if !strings.Contains(resp.Result, want) {
			t.Fatalf("prompt = %q, want substring %q", resp.Result, want)
		}
	}
	if strings.Contains(resp.Result, "New request:") {
		t.Fatalf("prompt should not contain legacy replay marker: %q", resp.Result)
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

func TestAtomicSave(t *testing.T) {
	store := FilesystemStore{Dir: t.TempDir()}
	original := threadFixture("atomic", "before", []Turn{{Role: "user", Content: "hello"}}, map[string]string{"a": "s1"})
	if err := store.SaveThread(original); err != nil {
		t.Fatalf("initial save: %v", err)
	}

	path := filepath.Join(store.Dir, "atomic.json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(`{"id":"atomic","turns":[`), 0o644); err != nil {
		t.Fatalf("write stale temp file: %v", err)
	}

	loaded, err := store.LoadThread("atomic")
	if err != nil {
		t.Fatalf("load with stale temp file: %v", err)
	}
	assertThreadSnapshot(t, loaded, "before", "s1", 1)

	updated := threadFixture(
		"atomic",
		"after",
		append(loaded.Turns, Turn{Role: "assistant", Content: "world"}),
		map[string]string{"a": "s2"},
	)
	if err := store.SaveThread(updated); err != nil {
		t.Fatalf("save with stale temp file present: %v", err)
	}

	after, err := store.LoadThread("atomic")
	if err != nil {
		t.Fatalf("load after atomic save: %v", err)
	}
	assertThreadSnapshot(t, after, "after", "s2", 2)
	assertMissingFile(t, tmpPath)
}

func assertThreadFileExists(t *testing.T, id string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(ThreadDir(), id+".json")); err != nil {
		t.Fatalf("thread file not created: %v", err)
	}
}

func assertTurn(t *testing.T, turn Turn, role, content, source string) {
	t.Helper()
	if turn.Role != role || turn.Content != content {
		t.Fatalf("turn = %+v, want role=%q content=%q", turn, role, content)
	}
	if turn.Source != source {
		t.Fatalf("turn source = %q, want %q", turn.Source, source)
	}
}

func threadFixture(id, summary string, turns []Turn, sessions map[string]string) *Thread {
	return &Thread{
		ID:             id,
		Summary:        summary,
		Turns:          turns,
		NativeSessions: sessions,
	}
}

func assertThreadSnapshot(t *testing.T, thread *Thread, summary, session string, turns int) {
	t.Helper()
	if thread.Summary != summary || thread.NativeSessions["a"] != session || len(thread.Turns) != turns {
		t.Fatalf("unexpected thread: %+v", thread)
	}
}

func assertMissingFile(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("file should be absent, stat err = %v", err)
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
			{Role: "user", Content: "hi", Backend: "a", Source: "cron-nightly", TS: "2025-01-01T00:00:00Z"},
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
	if got.Turns[0].Source != "cron-nightly" {
		t.Fatalf("round trip mismatch: %+v", got)
	}
}
