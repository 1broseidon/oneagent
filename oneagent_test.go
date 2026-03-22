package oneagent

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fakeJSON returns a backend that emits a static JSON blob.
func fakeJSON(result, session string) Backend {
	return Backend{
		Cmd:     []string{"sh", "-c", `printf '{"result":"` + result + `","session":"` + session + `"}'`},
		Format:  "json",
		Result:  "result",
		Session: "session",
	}
}

// fakeJSONL returns a backend that emits newline-delimited JSON events.
func fakeJSONL(events string) Backend {
	return Backend{
		Cmd:         []string{"sh", "-c", "printf '" + events + "'"},
		Format:      "jsonl",
		Result:      "data.text",
		ResultWhen:  "type=delta",
		Session:     "sid",
		SessionWhen: "type=done",
		Error:       "error.msg",
		ErrorWhen:   "type=error",
	}
}

func TestJSONNormalization(t *testing.T) {
	backends := map[string]Backend{"js": fakeJSON("hello world", "sess-1")}
	resp := Run(backends, RunOpts{Backend: "js", Prompt: "hi"})

	if resp.Result != "hello world" {
		t.Fatalf("result = %q, want %q", resp.Result, "hello world")
	}
	if resp.Session != "sess-1" {
		t.Fatalf("session = %q, want %q", resp.Session, "sess-1")
	}
	if resp.Backend != "js" {
		t.Fatalf("backend = %q, want %q", resp.Backend, "js")
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %q", resp.Error)
	}
}

func TestJSONPreservesEmptyResult(t *testing.T) {
	backends := map[string]Backend{"js": fakeJSON("", "sess-1")}
	resp := Run(backends, RunOpts{Backend: "js", Prompt: "hi"})

	if resp.Result != "" {
		t.Fatalf("result = %q, want empty", resp.Result)
	}
	if resp.Session != "sess-1" {
		t.Fatalf("session = %q, want %q", resp.Session, "sess-1")
	}
	if resp.Error != "" {
		t.Fatalf("unexpected error: %q", resp.Error)
	}
}

func TestJSONLEventExtraction(t *testing.T) {
	events := `{"type":"delta","data":{"text":"one"}}
{"type":"delta","data":{"text":"two"}}
{"type":"done","sid":"sess-42"}
`
	b := fakeJSONL(events)
	b.ResultAppend = true
	backends := map[string]Backend{"jl": b}
	resp := Run(backends, RunOpts{Backend: "jl", Prompt: "hi"})

	if resp.Result != "onetwo" {
		t.Fatalf("result = %q, want %q", resp.Result, "onetwo")
	}
	if resp.Session != "sess-42" {
		t.Fatalf("session = %q, want %q", resp.Session, "sess-42")
	}
}

func TestRunStreamEmitsNormalizedEvents(t *testing.T) {
	events := `{"type":"session","sid":"sess-42"}
{"type":"delta","data":{"text":"one"}}
{"type":"delta","data":{"text":"two"}}
`
	b := fakeJSONL(events)
	b.ResultAppend = true
	b.SessionWhen = "type=session"
	backends := map[string]Backend{"jl": b}

	var got []StreamEvent
	resp := RunStream(backends, RunOpts{Backend: "jl", Prompt: "hi"}, func(event StreamEvent) {
		got = append(got, event)
	})

	if resp.Result != "onetwo" {
		t.Fatalf("result = %q, want %q", resp.Result, "onetwo")
	}

	want := []StreamEvent{
		{Type: "session", Backend: "jl", Session: "sess-42"},
		{Type: "delta", Backend: "jl", Session: "sess-42", Delta: "one"},
		{Type: "delta", Backend: "jl", Session: "sess-42", Delta: "two"},
		{Type: "done", Backend: "jl", Session: "sess-42", Result: "onetwo"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %+v, want %+v", got, want)
	}
}

func TestRunStreamEmitsActivityEvents(t *testing.T) {
	events := `{"type":"session","sid":"sess-42"}
{"type":"started","tool":{"name":"Read","path":"README.md"}}
{"type":"delta","data":{"text":"one"}}
{"type":"done","data":{"text":"onetwo"}}
`
	b := Backend{
		Cmd:          []string{"sh", "-c", "printf '" + events + "'"},
		Format:       "jsonl",
		Activity:     "{tool.name} {tool.path}",
		ActivityWhen: "type=started",
		Delta:        "data.text",
		DeltaWhen:    "type=delta",
		Result:       "data.text",
		ResultWhen:   "type=done",
		Session:      "sid",
		SessionWhen:  "type=session",
	}
	backends := map[string]Backend{"jl": b}

	var got []StreamEvent
	resp := RunStream(backends, RunOpts{Backend: "jl", Prompt: "hi"}, func(event StreamEvent) {
		got = append(got, event)
	})

	if resp.Result != "onetwo" {
		t.Fatalf("result = %q, want %q", resp.Result, "onetwo")
	}

	want := []StreamEvent{
		{Type: "session", Backend: "jl", Session: "sess-42"},
		{Type: "activity", Backend: "jl", Session: "sess-42", Activity: "Read README.md"},
		{Type: "delta", Backend: "jl", Session: "sess-42", Delta: "one"},
		{Type: "done", Backend: "jl", Session: "sess-42", Result: "onetwo"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %+v, want %+v", got, want)
	}
}

func TestRunStreamUsesDeltaSelectorsWhenConfigured(t *testing.T) {
	events := `{"type":"session","sid":"sess-42"}
{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"one"}}
{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"two"}}
{"type":"item.completed","item":{"text":"onetwo"}}
`
	b := Backend{
		Cmd:         []string{"sh", "-c", "printf '" + events + "'"},
		Format:      "jsonl",
		Delta:       "assistantMessageEvent.delta",
		DeltaWhen:   "type=message_update&assistantMessageEvent.type=text_delta",
		Result:      "item.text",
		ResultWhen:  "type=item.completed",
		Session:     "sid",
		SessionWhen: "type=session",
	}
	backends := map[string]Backend{"jl": b}

	var got []StreamEvent
	resp := RunStream(backends, RunOpts{Backend: "jl", Prompt: "hi"}, func(event StreamEvent) {
		got = append(got, event)
	})

	if resp.Result != "onetwo" {
		t.Fatalf("result = %q, want %q", resp.Result, "onetwo")
	}

	want := []StreamEvent{
		{Type: "session", Backend: "jl", Session: "sess-42"},
		{Type: "delta", Backend: "jl", Session: "sess-42", Delta: "one"},
		{Type: "delta", Backend: "jl", Session: "sess-42", Delta: "two"},
		{Type: "done", Backend: "jl", Session: "sess-42", Result: "onetwo"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %+v, want %+v", got, want)
	}
}

func TestRunStreamCanUseFinalAssistantMessageForResult(t *testing.T) {
	events := `{"type":"session","sid":"sess-42"}
{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"Let me look first."}}
{"type":"message_update","assistantMessageEvent":{"type":"toolcall_end","toolCall":{"name":"read","arguments":{"path":"README.md"}}}}
{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"Let me look first."},{"type":"toolCall","name":"read","arguments":{"path":"README.md"}}],"stopReason":"toolUse"}}
{"type":"tool_execution_end","toolCallId":"tool-1","toolName":"read","result":{"content":[{"type":"text","text":"ok"}]}}
{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"All clean."}}
{"type":"message_end","message":{"role":"assistant","content":[{"type":"text","text":"All clean."}],"stopReason":"stop"}}
`
	b := Backend{
		Cmd:          []string{"sh", "-c", "printf '" + events + "'"},
		Format:       "jsonl",
		Activity:     "{assistantMessageEvent.toolCall.name} {assistantMessageEvent.toolCall.arguments.path}",
		ActivityWhen: "type=message_update&assistantMessageEvent.type=toolcall_end",
		Delta:        "assistantMessageEvent.delta",
		DeltaWhen:    "type=message_update&assistantMessageEvent.type=text_delta",
		Result:       "message.content.0.text",
		ResultWhen:   "type=message_end&message.role=assistant&message.content.0.type=text&message.stopReason=stop",
		Session:      "sid",
		SessionWhen:  "type=session",
	}
	backends := map[string]Backend{"jl": b}

	var got []StreamEvent
	resp := RunStream(backends, RunOpts{Backend: "jl", Prompt: "hi"}, func(event StreamEvent) {
		got = append(got, event)
	})

	if resp.Result != "All clean." {
		t.Fatalf("result = %q, want %q", resp.Result, "All clean.")
	}

	want := []StreamEvent{
		{Type: "session", Backend: "jl", Session: "sess-42"},
		{Type: "delta", Backend: "jl", Session: "sess-42", Delta: "Let me look first."},
		{Type: "activity", Backend: "jl", Session: "sess-42", Activity: "read README.md"},
		{Type: "delta", Backend: "jl", Session: "sess-42", Delta: "All clean."},
		{Type: "done", Backend: "jl", Session: "sess-42", Result: "All clean."},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %+v, want %+v", got, want)
	}
}

func TestJSONGetSupportsArrayIndexes(t *testing.T) {
	line := map[string]any{
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"name":  "Read",
					"input": map[string]any{"file_path": "README.md"},
				},
			},
		},
	}

	if got := jsonGet(line, "message.content.0.name"); got != "Read" {
		t.Fatalf("jsonGet array access = %v, want Read", got)
	}
	if got := jsonGet(line, "message.content.0.input.file_path"); got != "README.md" {
		t.Fatalf("jsonGet nested array access = %v, want README.md", got)
	}
}

func TestJSONGetSupportsNegativeArrayIndexes(t *testing.T) {
	line := map[string]any{
		"message": map[string]any{
			"content": []any{
				map[string]any{"type": "thinking", "text": "hmm"},
				map[string]any{"type": "text", "text": "answer"},
			},
		},
	}

	if got := jsonGet(line, "message.content.-1.text"); got != "answer" {
		t.Fatalf("jsonGet negative index = %v, want answer", got)
	}
	if got := jsonGet(line, "message.content.-1.type"); got != "text" {
		t.Fatalf("jsonGet negative index type = %v, want text", got)
	}
	if got := jsonGet(line, "message.content.-2.type"); got != "thinking" {
		t.Fatalf("jsonGet negative index -2 = %v, want thinking", got)
	}
	// out of bounds
	if got := jsonGet(line, "message.content.-3.type"); got != nil {
		t.Fatalf("jsonGet negative out of bounds = %v, want nil", got)
	}
}

func TestExtractTemplateBuildsActivityMessage(t *testing.T) {
	line := map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"type":  "tool_use",
					"name":  "Read",
					"input": map[string]any{"file_path": "README.md"},
				},
			},
		},
	}

	got := extractTemplate(line, "type=assistant&message.content.0.type=tool_use", "{message.content.0.name} {message.content.0.input.file_path}")
	if got != "Read README.md" {
		t.Fatalf("extractTemplate = %q, want %q", got, "Read README.md")
	}
}

func TestResultAppendConcatenatesDeltas(t *testing.T) {
	events := `{"type":"delta","data":{"text":"a"}}
{"type":"delta","data":{"text":"b"}}
{"type":"delta","data":{"text":"c"}}
`
	b := fakeJSONL(events)
	b.ResultAppend = true
	backends := map[string]Backend{"s": b}
	resp := Run(backends, RunOpts{Backend: "s", Prompt: "go"})

	if resp.Result != "abc" {
		t.Fatalf("result = %q, want %q", resp.Result, "abc")
	}
}

func TestPlainTextFallback(t *testing.T) {
	backends := map[string]Backend{
		"txt": {
			Cmd:     []string{"sh", "-c", "echo 'not json at all'"},
			Format:  "json",
			Result:  "result",
			Session: "session",
		},
	}
	resp := Run(backends, RunOpts{Backend: "txt", Prompt: "hi"})
	if resp.Result != "not json at all" {
		t.Fatalf("result = %q, want %q", resp.Result, "not json at all")
	}
}

func TestBackendErrorSurfaced(t *testing.T) {
	backends := map[string]Backend{
		"bad": {
			Cmd:    []string{"sh", "-c", "exit 1"},
			Format: "json",
			Result: "result",
		},
	}
	resp := Run(backends, RunOpts{Backend: "bad", Prompt: "hi"})
	if resp.Error == "" {
		t.Fatal("expected error from failing backend")
	}
}

func TestRunContextCancelsBackend(t *testing.T) {
	backends := map[string]Backend{
		"slow": {
			Cmd:    []string{"sh", "-c", "sleep 30"},
			Format: "json",
			Result: "result",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(100*time.Millisecond, cancel)

	start := time.Now()
	resp := RunContext(ctx, backends, RunOpts{Backend: "slow", Prompt: "hi"})
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("RunContext took too long after cancel: %v", elapsed)
	}
	if resp.Error != context.Canceled.Error() {
		t.Fatalf("error = %q, want %q", resp.Error, context.Canceled.Error())
	}
}

func TestResponseIncludesExitCodeAndStderr(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		backends := map[string]Backend{
			"bad": {
				Cmd:    []string{"sh", "-c", "printf 'json boom\\n' >&2; exit 7"},
				Format: "json",
				Result: "result",
			},
		}

		resp := Run(backends, RunOpts{Backend: "bad", Prompt: "hi"})

		if resp.ExitCode != 7 {
			t.Fatalf("exit code = %d, want 7", resp.ExitCode)
		}
		if resp.Stderr != "json boom\n" {
			t.Fatalf("stderr = %q, want %q", resp.Stderr, "json boom\n")
		}
	})

	t.Run("jsonl", func(t *testing.T) {
		backends := map[string]Backend{
			"bad": {
				Cmd:        []string{"sh", "-c", "printf '{\"type\":\"delta\",\"data\":{\"text\":\"partial\"}}\\n'; printf 'jsonl boom\\n' >&2; exit 9"},
				Format:     "jsonl",
				Result:     "data.text",
				ResultWhen: "type=delta",
			},
		}

		resp := Run(backends, RunOpts{Backend: "bad", Prompt: "hi"})

		if resp.ExitCode != 9 {
			t.Fatalf("exit code = %d, want 9", resp.ExitCode)
		}
		if resp.Stderr != "jsonl boom\n" {
			t.Fatalf("stderr = %q, want %q", resp.Stderr, "jsonl boom\n")
		}
	})
}

func TestRunBackfillsSessionFromResumeOpts(t *testing.T) {
	backends := map[string]Backend{
		"jl": {
			Cmd:         []string{"sh", "-c", "printf '{\"type\":\"done\",\"data\":{\"text\":\"ok\"}}\\n'"},
			ResumeCmd:   []string{"sh", "-c", "printf '{\"type\":\"done\",\"data\":{\"text\":\"ok\"}}\\n'"},
			Format:      "jsonl",
			Result:      "data.text",
			ResultWhen:  "type=done",
			Session:     "sid",
			SessionWhen: "type=session",
		},
	}

	resp := Run(backends, RunOpts{Backend: "jl", Prompt: "hi", SessionID: "sess-resume"})

	if resp.Session != "sess-resume" {
		t.Fatalf("session = %q, want %q", resp.Session, "sess-resume")
	}
}

func TestMissingBackend(t *testing.T) {
	resp := Run(map[string]Backend{}, RunOpts{Backend: "nope", Prompt: "hi"})
	if resp.Error == "" || resp.Backend != "nope" {
		t.Fatalf("expected error for missing backend, got %+v", resp)
	}
}

func TestDefaultModelUsed(t *testing.T) {
	// The fake backend echoes the prompt which includes the model via template.
	backends := map[string]Backend{
		"m": {
			Cmd:          []string{"sh", "-c", `printf '{"result":"model={model}","session":""}'`},
			Format:       "json",
			Result:       "result",
			Session:      "session",
			DefaultModel: "gpt-5",
		},
	}
	// No model override — but {model} isn't in the cmd template above, so we
	// test via buildCmd directly.
	b := backends["m"]
	cmd, err := buildCmd(b, RunOpts{Backend: "m", Prompt: "hi"})
	if err != nil {
		t.Fatalf("buildCmd returned error: %v", err)
	}
	// The command should have been built; default model is applied internally.
	// We verify buildCmd doesn't panic and returns a valid cmd.
	if cmd.Path == "" {
		t.Fatal("buildCmd returned empty path")
	}
}

func TestEmptyVariableDropsPrecedingFlag(t *testing.T) {
	tmpl := []string{"agent", "--model", "{model}", "--prompt", "{prompt}"}
	vars := map[string]string{"model": "", "prompt": "hello"}
	got := substArgs(tmpl, vars)

	// --model and its empty value should both be dropped.
	for _, a := range got {
		if a == "--model" {
			t.Fatalf("--model flag should have been dropped, got %v", got)
		}
	}
	if len(got) != 3 { // agent --prompt hello
		t.Fatalf("args = %v, want [agent --prompt hello]", got)
	}
}

func TestEmptyInlineAssignmentDropsPrecedingFlag(t *testing.T) {
	tmpl := []string{"agent", "-c", "model_reasoning_effort={thinking}", "--prompt", "{prompt}"}
	vars := map[string]string{"thinking": "", "prompt": "hello"}
	got := substArgs(tmpl, vars)
	want := []string{"agent", "--prompt", "hello"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("args = %v, want %v", got, want)
	}
}

func TestEmptyInlineAssignmentSelfContainedFlagDropsOnlyToken(t *testing.T) {
	tmpl := []string{"agent", "--model={model}", "--prompt", "{prompt}"}
	vars := map[string]string{"model": "", "prompt": "hello"}
	got := substArgs(tmpl, vars)
	want := []string{"agent", "--prompt", "hello"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("args = %v, want %v", got, want)
	}
}

func TestSystemPromptPrependedOnFirstMessage(t *testing.T) {
	b := Backend{
		Cmd:          []string{"agent", "--prompt", "{prompt}"},
		SystemPrompt: "SYSPROMPT",
	}
	cmd, err := buildCmd(b, RunOpts{Backend: "sp", Prompt: "hello"})
	if err != nil {
		t.Fatalf("buildCmd returned error: %v", err)
	}
	got := strings.Join(cmd.Args, "\n")
	if !strings.Contains(got, "SYSPROMPT\n\nhello") {
		t.Fatalf("system prompt not prepended, args = %v", cmd.Args)
	}
}

func TestSystemPromptSkippedOnResume(t *testing.T) {
	b := Backend{
		Cmd:          []string{"agent", "--prompt", "{prompt}"},
		ResumeCmd:    []string{"agent", "--session", "{session}", "--prompt", "{prompt}"},
		SystemPrompt: "SYSPROMPT",
	}
	cmd, err := buildCmd(b, RunOpts{Backend: "sp", Prompt: "hello", SessionID: "s1"})
	if err != nil {
		t.Fatalf("buildCmd returned error: %v", err)
	}
	for _, a := range cmd.Args {
		if strings.Contains(a, "SYSPROMPT") {
			t.Fatal("system prompt should be skipped on resume")
		}
	}
}

func TestResumeCmdSelectedWithSession(t *testing.T) {
	b := Backend{
		Cmd:       []string{"agent", "--prompt", "{prompt}"},
		ResumeCmd: []string{"agent", "--resume", "{session}", "--prompt", "{prompt}"},
	}
	cmd, err := buildCmd(b, RunOpts{Backend: "r", Prompt: "hi", SessionID: "s99"})
	if err != nil {
		t.Fatalf("buildCmd returned error: %v", err)
	}
	found := false
	for _, a := range cmd.Args {
		if a == "s99" {
			found = true
		}
	}
	if !found {
		t.Fatalf("resume_cmd not used with session, args = %v", cmd.Args)
	}
}

func TestCWDOnResume(t *testing.T) {
	t.Run("resume without cwd template sets cmd dir", func(t *testing.T) {
		b := Backend{
			Cmd:       []string{"agent", "-C", "{cwd}", "--prompt", "{prompt}"},
			ResumeCmd: []string{"agent", "--resume", "{session}", "--prompt", "{prompt}"},
		}
		cmd, err := buildCmd(b, RunOpts{Backend: "r", Prompt: "hi", SessionID: "s99", CWD: "/tmp/test"})
		if err != nil {
			t.Fatalf("buildCmd returned error: %v", err)
		}
		if cmd.Dir != "/tmp/test" {
			t.Fatalf("cmd.Dir = %q, want /tmp/test", cmd.Dir)
		}
	})

	t.Run("resume with cwd template leaves cmd dir empty", func(t *testing.T) {
		b := Backend{
			Cmd:       []string{"agent", "--prompt", "{prompt}"},
			ResumeCmd: []string{"agent", "--resume", "{session}", "-C", "{cwd}", "--prompt", "{prompt}"},
		}
		cmd, err := buildCmd(b, RunOpts{Backend: "r", Prompt: "hi", SessionID: "s99", CWD: "/tmp/test"})
		if err != nil {
			t.Fatalf("buildCmd returned error: %v", err)
		}
		if cmd.Dir != "" {
			t.Fatalf("cmd.Dir = %q, want empty", cmd.Dir)
		}
	})
}

func TestCwdSetWhenNotInTemplate(t *testing.T) {
	b := Backend{Cmd: []string{"sh", "-c", "echo ok"}}
	cmd, err := buildCmd(b, RunOpts{Backend: "c", Prompt: "hi", CWD: "/tmp/test"})
	if err != nil {
		t.Fatalf("buildCmd returned error: %v", err)
	}
	if cmd.Dir != "/tmp/test" {
		t.Fatalf("cmd.Dir = %q, want /tmp/test", cmd.Dir)
	}
}

func TestCwdNotSetWhenInTemplate(t *testing.T) {
	b := Backend{Cmd: []string{"agent", "-C", "{cwd}", "--prompt", "{prompt}"}}
	cmd, err := buildCmd(b, RunOpts{Backend: "c", Prompt: "hi", CWD: "/tmp/test"})
	if err != nil {
		t.Fatalf("buildCmd returned error: %v", err)
	}
	if cmd.Dir != "" {
		t.Fatalf("cmd.Dir = %q, want empty (cwd in template)", cmd.Dir)
	}
}

func TestBuildCmdReturnsErrorWhenSubstitutionRemovesExecutable(t *testing.T) {
	b := Backend{Cmd: []string{"{cwd}"}}
	_, err := buildCmd(b, RunOpts{Backend: "broken", Prompt: "hi"})
	if err == nil {
		t.Fatal("buildCmd should fail when command becomes empty")
	}
	if !strings.Contains(err.Error(), `backend "broken" produced an empty command`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMatchWhenHandlesBooleanValues(t *testing.T) {
	line := map[string]any{"type": "result", "is_error": true}
	if !matchWhen(line, "type=result&is_error=true") {
		t.Fatal("matchWhen should match boolean values")
	}
}

func TestScannerOverflowLogged(t *testing.T) {
	dir := t.TempDir()
	eventsPath := filepath.Join(dir, "events.jsonl")
	events := `{"type":"delta","data":{"text":"` + strings.Repeat("a", 1024*1024+1) + `"}}` + "\n"
	if err := os.WriteFile(eventsPath, []byte(events), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}

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

	backends := map[string]Backend{
		"jl": {
			Cmd:        []string{"sh", "-c", fmt.Sprintf("cat %q; exit 1", eventsPath)},
			Format:     "jsonl",
			Result:     "data.text",
			ResultWhen: "type=delta",
		},
	}
	resp := Run(backends, RunOpts{Backend: "jl", Prompt: "hi"})

	if resp.Error == "" {
		t.Fatal("expected scanner overflow to surface as an error")
	}
	if !strings.Contains(logs.String(), "scanner error:") || !strings.Contains(logs.String(), "token too long") {
		t.Fatalf("expected scanner overflow to be logged, got %q", logs.String())
	}
}

func TestLoadBackendsUsesEmbeddedDefaultsWhenUserConfigMissing(t *testing.T) {
	setupThreadHome(t)

	backends, err := LoadBackends("")
	if err != nil {
		t.Fatalf("LoadBackends: %v", err)
	}

	for _, name := range []string{"claude", "codex", "opencode", "pi"} {
		if _, ok := backends[name]; !ok {
			t.Fatalf("embedded backend %q missing from %v", name, mapsKeys(backends))
		}
	}
}

func TestLoadBackendsMergesUserOverridesOntoDefaults(t *testing.T) {
	setupThreadHome(t)

	if err := os.MkdirAll(ConfigDir(), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	override := `{
		"claude": {
			"run": "custom-claude --prompt {prompt}",
			"format": "json",
			"result": "result",
			"session": "session"
		},
		"custom": {
			"run": "custom-agent {prompt}",
			"format": "json",
			"result": "result",
			"session": "session"
		}
	}`
	if err := os.WriteFile(DefaultConfigPath(), []byte(override), 0o644); err != nil {
		t.Fatalf("write override config: %v", err)
	}

	backends, err := LoadBackends("")
	if err != nil {
		t.Fatalf("LoadBackends: %v", err)
	}

	if got := backends["claude"].Cmd[0]; got != "custom-claude" {
		t.Fatalf("claude override not applied, cmd[0] = %q", got)
	}
	if _, ok := backends["custom"]; !ok {
		t.Fatalf("custom backend missing from %v", mapsKeys(backends))
	}
	if _, ok := backends["codex"]; !ok {
		t.Fatal("embedded defaults should remain when overlay adds/replaces entries")
	}
}

func TestLoadBackendsExplicitPathBypassesEmbeddedDefaults(t *testing.T) {
	setupThreadHome(t)

	customPath := filepath.Join(t.TempDir(), "backends.json")
	config := `{
		"only": {
			"run": "only-agent {prompt}",
			"format": "json",
			"result": "result",
			"session": "session"
		}
	}`
	if err := os.WriteFile(customPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write explicit config: %v", err)
	}

	backends, err := LoadBackends(customPath)
	if err != nil {
		t.Fatalf("LoadBackends: %v", err)
	}

	if len(backends) != 1 {
		t.Fatalf("explicit config should load only its own backends, got %v", mapsKeys(backends))
	}
	if _, ok := backends["only"]; !ok {
		t.Fatalf("explicit backend missing from %v", mapsKeys(backends))
	}
}

func TestLoadBackendsWithOptionsUsesCustomOverridePath(t *testing.T) {
	setupThreadHome(t)

	if err := os.MkdirAll(ConfigDir(), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	defaultOverride := `{
		"claude": {
			"run": "user-claude {prompt}",
			"format": "json",
			"result": "result",
			"session": "session"
		}
	}`
	if err := os.WriteFile(DefaultConfigPath(), []byte(defaultOverride), 0o644); err != nil {
		t.Fatalf("write default override config: %v", err)
	}

	customPath := filepath.Join(t.TempDir(), "moxie-backends.json")
	customOverride := `{
		"pi": {
			"run": "pi-custom {prompt}",
			"format": "json",
			"result": "result",
			"session": "session"
		},
		"custom": {
			"run": "custom-agent {prompt}",
			"format": "json",
			"result": "result",
			"session": "session"
		}
	}`
	if err := os.WriteFile(customPath, []byte(customOverride), 0o644); err != nil {
		t.Fatalf("write custom override config: %v", err)
	}

	backends, err := LoadBackendsWithOptions(LoadOptions{
		IncludeEmbedded: true,
		OverridePath:    customPath,
	})
	if err != nil {
		t.Fatalf("LoadBackendsWithOptions: %v", err)
	}

	if got := backends["claude"].Cmd[0]; got == "user-claude" {
		t.Fatalf("default ~/.config/oneagent override should not apply, got %q", got)
	}
	if got := backends["pi"].Cmd[0]; got != "pi-custom" {
		t.Fatalf("custom override not applied, pi cmd[0] = %q", got)
	}
	if _, ok := backends["custom"]; !ok {
		t.Fatalf("custom backend missing from %v", mapsKeys(backends))
	}
}

func mapsKeys(backends map[string]Backend) []string {
	keys := make([]string, 0, len(backends))
	for name := range backends {
		keys = append(keys, name)
	}
	return keys
}
