// Package oneagent provides a config-driven interface for running any AI agent CLI.
//
// Backends are defined in a JSON config with command templates, output format
// (json/jsonl), and field paths for extracting results, sessions, and errors.
// Template variables ({prompt}, {model}, {cwd}, {session}) are substituted at
// runtime. This lets you add new agent backends without writing any code.
package oneagent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Backend defines how to invoke and parse output from an agent CLI.
type Backend struct {
	Cmd          []string `json:"cmd"`
	ResumeCmd    []string `json:"resume_cmd,omitempty"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
	Format       string   `json:"format"` // "json" or "jsonl"
	Result       string   `json:"result"`
	ResultWhen   string   `json:"result_when,omitempty"`
	ResultAppend bool     `json:"result_append,omitempty"`
	Session      string   `json:"session"`
	SessionWhen  string   `json:"session_when,omitempty"`
	Error        string   `json:"error,omitempty"`
	ErrorWhen    string   `json:"error_when,omitempty"`
	DefaultModel string   `json:"default_model,omitempty"`
}

// Response is the normalized output from any backend.
type Response struct {
	Result   string `json:"result"`
	Session  string `json:"session"`
	ThreadID string `json:"thread_id,omitempty"`
	Backend  string `json:"backend"`
	Error    string `json:"error,omitempty"`
}

// RunOpts configures a single agent invocation.
type RunOpts struct {
	Backend   string
	Prompt    string
	Model     string
	CWD       string
	SessionID string
	ThreadID  string
}

// ConfigDir returns the default config directory (~/.config/oneagent).
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "oneagent")
}

// LoadBackends reads backend definitions from a JSON file.
func LoadBackends(path string) (map[string]Backend, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var backends map[string]Backend
	if err := json.Unmarshal(data, &backends); err != nil {
		return nil, fmt.Errorf("invalid backends config: %w", err)
	}
	return backends, nil
}

// Run executes a prompt against the specified backend and returns a normalized response.
func buildCmd(b Backend, opts RunOpts) *exec.Cmd {
	model := opts.Model
	if model == "" {
		model = b.DefaultModel
	}

	prompt := opts.Prompt
	if opts.SessionID == "" && b.SystemPrompt != "" {
		prompt = b.SystemPrompt + "\n\n" + opts.Prompt
	}

	vars := map[string]string{
		"prompt":  prompt,
		"model":   model,
		"cwd":     opts.CWD,
		"session": opts.SessionID,
	}

	tmpl := b.Cmd
	if opts.SessionID != "" && len(b.ResumeCmd) > 0 {
		tmpl = b.ResumeCmd
	}
	args := substArgs(tmpl, vars)

	cmd := exec.Command(args[0], args[1:]...)
	if opts.CWD != "" && !containsVar(b.Cmd, "{cwd}") {
		cmd.Dir = opts.CWD
	}
	cmd.Env = os.Environ()
	return cmd
}

// Run executes a prompt against the specified backend and returns a normalized response.
func Run(backends map[string]Backend, opts RunOpts) Response {
	b, ok := backends[opts.Backend]
	if !ok {
		return Response{Error: "backend not configured: " + opts.Backend, Backend: opts.Backend}
	}

	cmd := buildCmd(b, opts)

	var result, session string
	var err error

	switch b.Format {
	case "jsonl":
		result, session, err = runJSONL(cmd, b)
	default:
		result, session, err = runJSON(cmd, b)
	}

	resp := Response{Result: result, Session: session, Backend: opts.Backend}
	if err != nil {
		log.Printf("%s error: %v", opts.Backend, err)
		if result != "" {
			resp.Error = result
		} else {
			resp.Error = err.Error()
		}
		return resp
	}

	if resp.Result == "" {
		resp.Result = "Done — nothing to report."
	}
	return resp
}

// substArgs replaces {variables} in a command template.
// When a variable resolves to empty, the preceding flag is also dropped.
func substArgs(tmpl []string, vars map[string]string) []string {
	out := make([]string, 0, len(tmpl))
	for _, t := range tmpl {
		val := t
		for k, v := range vars {
			val = strings.ReplaceAll(val, "{"+k+"}", v)
		}
		if val == "" {
			if len(out) > 0 && strings.HasPrefix(out[len(out)-1], "-") {
				out = out[:len(out)-1]
			}
			continue
		}
		out = append(out, val)
	}
	return out
}

func containsVar(tmpl []string, v string) bool {
	for _, t := range tmpl {
		if strings.Contains(t, v) {
			return true
		}
	}
	return false
}

func runJSON(cmd *exec.Cmd, b Backend) (result, session string, err error) {
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Printf("stderr: %s", exitErr.Stderr)
		}
		return "", "", err
	}

	var blob map[string]any
	if err := json.Unmarshal(out, &blob); err != nil {
		return strings.TrimSpace(string(out)), "", nil
	}

	if b.ErrorWhen != "" && matchWhen(blob, b.ErrorWhen) {
		errMsg, _ := jsonGet(blob, b.Error).(string)
		if errMsg != "" {
			return errMsg, "", fmt.Errorf("%s", errMsg)
		}
	}
	result, _ = jsonGet(blob, b.Result).(string)
	session, _ = jsonGet(blob, b.Session).(string)
	return result, session, nil
}

func runJSONL(cmd *exec.Cmd, b Backend) (result, session string, err error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", "", err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	result, session, lastErr := scanJSONL(scanner, b)

	if err = cmd.Wait(); err != nil {
		if s := stderr.String(); s != "" {
			log.Printf("stderr: %s", strings.TrimSpace(s))
		}
		if result == "" && lastErr != "" {
			result = lastErr
		}
	}
	return result, session, err
}

func extractField(line map[string]any, when, field string) string {
	if when != "" && matchWhen(line, when) {
		if v, _ := jsonGet(line, field).(string); v != "" {
			return v
		}
	}
	return ""
}

func scanJSONL(scanner *bufio.Scanner, b Backend) (result, session, lastErr string) {
	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if v := extractField(line, b.ErrorWhen, b.Error); v != "" {
			lastErr = v
		}
		if v := extractField(line, b.SessionWhen, b.Session); v != "" {
			session = v
		}
		if v := extractField(line, b.ResultWhen, b.Result); v != "" {
			if b.ResultAppend {
				result += v
			} else {
				result = v
			}
		}
	}
	return
}

// jsonGet walks a dot-separated path into a map.
func jsonGet(m map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var cur any = m
	for _, p := range parts {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = obj[p]
	}
	return cur
}

// matchWhen checks "key=value[&key=value...]" conditions against a JSON object.
func matchWhen(m map[string]any, when string) bool {
	for _, cond := range strings.Split(when, "&") {
		k, v, ok := strings.Cut(cond, "=")
		if !ok {
			return false
		}
		got, _ := jsonGet(m, k).(string)
		if got != v {
			return false
		}
	}
	return true
}
