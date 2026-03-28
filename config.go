package oneagent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// backendConfig is the JSON config format that compiles into Backend.
type backendConfig struct {
	Run          string   `json:"run"`
	Resume       string   `json:"resume,omitempty"`
	Model        string   `json:"model,omitempty"`
	System       string   `json:"system,omitempty"`
	Format       string   `json:"format,omitempty"`
	Activity     string   `json:"activity,omitempty"`
	ActivityWhen string   `json:"activity_when,omitempty"`
	Delta        string   `json:"delta,omitempty"`
	DeltaWhen    string   `json:"delta_when,omitempty"`
	Result       string   `json:"result,omitempty"`
	ResultWhen   string   `json:"result_when,omitempty"`
	ResultAppend bool     `json:"result_append,omitempty"`
	Session      string   `json:"session,omitempty"`
	SessionWhen  string   `json:"session_when,omitempty"`
	Error        string   `json:"error,omitempty"`
	ErrorWhen    string   `json:"error_when,omitempty"`
	Paths        []string `json:"paths,omitempty"`
	PromptStdin  bool     `json:"prompt_stdin,omitempty"`
	PreRun       string   `json:"pre_run,omitempty"`
	PostRun      string   `json:"post_run,omitempty"`
	Probe        string   `json:"probe,omitempty"`
}

// compileBackend converts a backendConfig into the canonical Backend struct.
func compileBackend(c backendConfig) (Backend, error) {
	b := Backend{
		SystemPrompt: c.System,
		Format:       c.Format,
		Activity:     c.Activity,
		ActivityWhen: c.ActivityWhen,
		Delta:        c.Delta,
		DeltaWhen:    c.DeltaWhen,
		Result:       c.Result,
		ResultWhen:   c.ResultWhen,
		ResultAppend: c.ResultAppend,
		Session:      c.Session,
		SessionWhen:  c.SessionWhen,
		Error:        c.Error,
		ErrorWhen:    c.ErrorWhen,
		DefaultModel: c.Model,
		Paths:        c.Paths,
		PromptStdin:  c.PromptStdin,
		PreRunCmd:    c.PreRun,
		PostRunCmd:   c.PostRun,
		Probe:        c.Probe,
	}

	if err := compileCommands(&b, c); err != nil {
		return b, err
	}
	return b, nil
}

// compileCommands compiles run/resume strings into cmd/resume_cmd arrays.
func compileCommands(b *Backend, c backendConfig) error {
	args, err := tokenize(c.Run)
	if err != nil {
		return fmt.Errorf("invalid run command: %w", err)
	}
	if len(args) == 0 {
		return fmt.Errorf("run command cannot be empty")
	}
	b.Cmd = args

	if c.Resume == "" {
		return nil
	}
	if strings.HasPrefix(c.Resume, "+ ") {
		extra, err := tokenize(c.Resume[2:])
		if err != nil {
			return fmt.Errorf("invalid resume patch: %w", err)
		}
		b.ResumeCmd = insertResume(b.Cmd, extra)
		return nil
	}
	resumeArgs, err := tokenize(c.Resume)
	if err != nil {
		return fmt.Errorf("invalid resume command: %w", err)
	}
	b.ResumeCmd = resumeArgs
	return nil
}

// insertResume builds a resume command by inserting extra args after {prompt} in cmd.
// If {prompt} is not found, extra args are appended.
func insertResume(cmd, extra []string) []string {
	out := make([]string, 0, len(cmd)+len(extra))
	inserted := false
	for _, arg := range cmd {
		out = append(out, arg)
		if arg == "{prompt}" && !inserted {
			out = append(out, extra...)
			inserted = true
		}
	}
	if !inserted {
		out = append(out, extra...)
	}
	return out
}

// tokenize splits a command string into argv, respecting single/double quotes and backslash escapes.
func tokenize(s string) ([]string, error) {
	runes := []rune(s)
	var args []string
	var cur strings.Builder
	var inSingle, inDouble bool

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\\' && !inSingle && i+1 < len(runes) {
			cur.WriteRune(runes[i+1])
			i++
			continue
		}
		inSingle, inDouble = tokenizeRune(r, inSingle, inDouble, &cur, &args)
	}

	if inSingle || inDouble {
		return nil, fmt.Errorf("unclosed quote in: %s", s)
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args, nil
}

// tokenizeRune processes one rune and returns updated quote state.
func tokenizeRune(r rune, inSingle, inDouble bool, cur *strings.Builder, args *[]string) (bool, bool) {
	if r == '\'' && !inDouble {
		return !inSingle, inDouble
	}
	if r == '"' && !inSingle {
		return inSingle, !inDouble
	}
	if r == ' ' && !inSingle && !inDouble {
		if cur.Len() > 0 {
			*args = append(*args, cur.String())
			cur.Reset()
		}
		return inSingle, inDouble
	}
	cur.WriteRune(r)
	return inSingle, inDouble
}

// loadConfigBackends unmarshals the config and compiles each entry.
func loadConfigBackends(data []byte) (map[string]Backend, error) {
	var raw map[string]backendConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid backends config: %w", err)
	}
	backends := make(map[string]Backend, len(raw))
	for name, c := range raw {
		b, err := compileBackend(c)
		if err != nil {
			return nil, fmt.Errorf("backend %s: %w", name, err)
		}
		backends[name] = b
	}
	return backends, nil
}
