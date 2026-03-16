package oneagent

import (
	"reflect"
	"testing"
)

func TestTokenizeSimple(t *testing.T) {
	got, err := tokenize("claude -p {prompt} --model {model}")
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []string{"claude", "-p", "{prompt}", "--model", "{model}"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTokenizeDoubleQuotes(t *testing.T) {
	got, err := tokenize(`sh -c "echo hello world"`)
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []string{"sh", "-c", "echo hello world"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTokenizeSingleQuotes(t *testing.T) {
	got, err := tokenize("sh -c 'echo hello world'")
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []string{"sh", "-c", "echo hello world"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTokenizeEscapedSpace(t *testing.T) {
	got, err := tokenize(`path\ with\ spaces arg2`)
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	want := []string{"path with spaces", "arg2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTokenizeUnclosedQuote(t *testing.T) {
	_, err := tokenize(`sh -c "unclosed`)
	if err == nil {
		t.Fatal("expected error for unclosed quote")
	}
}

func TestTokenizeEmpty(t *testing.T) {
	got, err := tokenize("")
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}

func TestCompileRunString(t *testing.T) {
	c := backendConfig{
		Run:    "claude -p {prompt} --output-format json",
		Format: "json",
		Result: "result",
	}
	b, err := compileBackend(c)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	want := []string{"claude", "-p", "{prompt}", "--output-format", "json"}
	if !reflect.DeepEqual(b.Cmd, want) {
		t.Fatalf("cmd = %v, want %v", b.Cmd, want)
	}
}

func TestCompileResumePatch(t *testing.T) {
	c := backendConfig{
		Run:     "claude -p {prompt} --model {model} --output-format json",
		Resume:  "+ --resume {session}",
		Format:  "json",
		Result:  "result",
		Session: "session_id",
	}
	b, err := compileBackend(c)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	// Extra args should be inserted after {prompt}.
	want := []string{"claude", "-p", "{prompt}", "--resume", "{session}", "--model", "{model}", "--output-format", "json"}
	if !reflect.DeepEqual(b.ResumeCmd, want) {
		t.Fatalf("resume_cmd = %v, want %v", b.ResumeCmd, want)
	}
}

func TestCompileResumePatchNoPrompt(t *testing.T) {
	// When {prompt} is not in the command, extra args are appended.
	c := backendConfig{
		Run:    "agent --auto",
		Resume: "+ --session {session}",
	}
	b, err := compileBackend(c)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	want := []string{"agent", "--auto", "--session", "{session}"}
	if !reflect.DeepEqual(b.ResumeCmd, want) {
		t.Fatalf("resume_cmd = %v, want %v", b.ResumeCmd, want)
	}
}

func TestCompileResumeFull(t *testing.T) {
	c := backendConfig{
		Run:    "codex exec {prompt} --json --full-auto",
		Resume: "codex exec resume {session} {prompt} --json --full-auto",
	}
	b, err := compileBackend(c)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	want := []string{"codex", "exec", "resume", "{session}", "{prompt}", "--json", "--full-auto"}
	if !reflect.DeepEqual(b.ResumeCmd, want) {
		t.Fatalf("resume_cmd = %v, want %v", b.ResumeCmd, want)
	}
}

func TestCompileModelAndSystem(t *testing.T) {
	c := backendConfig{
		Run:    "claude -p {prompt}",
		Model:  "sonnet",
		System: "You are helpful.",
	}
	b, err := compileBackend(c)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if b.DefaultModel != "sonnet" {
		t.Fatalf("model = %q, want sonnet", b.DefaultModel)
	}
	if b.SystemPrompt != "You are helpful." {
		t.Fatalf("system = %q, want 'You are helpful.'", b.SystemPrompt)
	}
}

func TestCompileAllFieldsPassThrough(t *testing.T) {
	c := backendConfig{
		Run:          "claude -p {prompt}",
		Resume:       "+ --resume {session}",
		System:       "sys",
		Format:       "json",
		Activity:     "{tool.name}",
		ActivityWhen: "type=tool",
		Delta:        "delta",
		DeltaWhen:    "type=delta",
		Result:       "result",
		Session:      "session_id",
		Model:        "opus",
		ResultAppend: true,
		ResultWhen:   "type=done",
		SessionWhen:  "type=start",
		Error:        "err",
		ErrorWhen:    "type=error",
	}
	b, err := compileBackend(c)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	assertEqual(t, "model", b.DefaultModel, "opus")
	assertEqual(t, "system", b.SystemPrompt, "sys")
	assertEqual(t, "activity", b.Activity, "{tool.name}")
	assertEqual(t, "activity_when", b.ActivityWhen, "type=tool")
	assertEqual(t, "delta", b.Delta, "delta")
	assertEqual(t, "delta_when", b.DeltaWhen, "type=delta")
	assertEqual(t, "result_when", b.ResultWhen, "type=done")
	assertEqual(t, "session_when", b.SessionWhen, "type=start")
	assertEqual(t, "error_when", b.ErrorWhen, "type=error")
	if !b.ResultAppend {
		t.Fatal("result_append should be true")
	}
}

func assertEqual(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}

func TestLoadCompactBackends(t *testing.T) {
	config := `{
		"test": {
			"run": "sh -c 'echo ok'",
			"format": "json",
			"result": "result",
			"model": "fast"
		}
	}`
	backends, err := loadConfigBackends([]byte(config))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	b, ok := backends["test"]
	if !ok {
		t.Fatal("backend 'test' not found")
	}
	if !reflect.DeepEqual(b.Cmd, []string{"sh", "-c", "echo ok"}) {
		t.Fatalf("cmd = %v", b.Cmd)
	}
	if b.DefaultModel != "fast" {
		t.Fatalf("model = %q", b.DefaultModel)
	}
}

func TestLoadCompactBackendsInvalidRun(t *testing.T) {
	config := `{"bad": {"run": "sh -c \"unclosed"}}`
	_, err := loadConfigBackends([]byte(config))
	if err == nil {
		t.Fatal("expected error for unclosed quote in run")
	}
}

func TestLoadCompactBackendsEmptyRun(t *testing.T) {
	config := `{"bad": {"run": ""}}`
	_, err := loadConfigBackends([]byte(config))
	if err == nil {
		t.Fatal("expected error for empty run command")
	}
}

func TestInsertResumeAfterPrompt(t *testing.T) {
	cmd := []string{"agent", "-p", "{prompt}", "--format", "json"}
	extra := []string{"--resume", "{session}"}
	got := insertResume(cmd, extra)
	want := []string{"agent", "-p", "{prompt}", "--resume", "{session}", "--format", "json"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestInsertResumeAppendWhenNoPrompt(t *testing.T) {
	cmd := []string{"agent", "--auto"}
	extra := []string{"--session", "{session}"}
	got := insertResume(cmd, extra)
	want := []string{"agent", "--auto", "--session", "{session}"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
