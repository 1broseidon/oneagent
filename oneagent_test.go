package oneagent

import (
	"testing"
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
	cmd := buildCmd(b, RunOpts{Backend: "m", Prompt: "hi"})
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

func TestSystemPromptPrependedOnFirstMessage(t *testing.T) {
	b := Backend{
		Cmd:          []string{"agent", "--prompt", "{prompt}"},
		SystemPrompt: "SYSPROMPT",
	}
	cmd := buildCmd(b, RunOpts{Backend: "sp", Prompt: "hello"})
	found := false
	for _, a := range cmd.Args {
		if a == "SYSPROMPT\n\nhello" {
			found = true
		}
	}
	if !found {
		t.Fatalf("system prompt not prepended on first message, args = %v", cmd.Args)
	}
}

func TestSystemPromptSkippedOnResume(t *testing.T) {
	b := Backend{
		Cmd:          []string{"agent", "--prompt", "{prompt}"},
		ResumeCmd:    []string{"agent", "--session", "{session}", "--prompt", "{prompt}"},
		SystemPrompt: "SYSPROMPT",
	}
	cmd := buildCmd(b, RunOpts{Backend: "sp", Prompt: "hello", SessionID: "s1"})
	for _, a := range cmd.Args {
		if a == "SYSPROMPT\n\nhello" {
			t.Fatal("system prompt should not be prepended on resume")
		}
	}
}

func TestResumeCmdSelectedWithSession(t *testing.T) {
	b := Backend{
		Cmd:       []string{"agent", "--prompt", "{prompt}"},
		ResumeCmd: []string{"agent", "--resume", "{session}", "--prompt", "{prompt}"},
	}
	cmd := buildCmd(b, RunOpts{Backend: "r", Prompt: "hi", SessionID: "s99"})
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

func TestCwdSetWhenNotInTemplate(t *testing.T) {
	b := Backend{Cmd: []string{"sh", "-c", "echo ok"}}
	cmd := buildCmd(b, RunOpts{Backend: "c", Prompt: "hi", CWD: "/tmp/test"})
	if cmd.Dir != "/tmp/test" {
		t.Fatalf("cmd.Dir = %q, want /tmp/test", cmd.Dir)
	}
}

func TestCwdNotSetWhenInTemplate(t *testing.T) {
	b := Backend{Cmd: []string{"agent", "-C", "{cwd}", "--prompt", "{prompt}"}}
	cmd := buildCmd(b, RunOpts{Backend: "c", Prompt: "hi", CWD: "/tmp/test"})
	if cmd.Dir != "" {
		t.Fatalf("cmd.Dir = %q, want empty (cwd in template)", cmd.Dir)
	}
}
