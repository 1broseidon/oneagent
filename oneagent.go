// Package oneagent provides a config-driven interface for running any AI agent CLI.
//
// Backends are defined in a compact JSON config with run/resume command strings,
// output format (json/jsonl), and field paths for extracting results, sessions,
// and errors. Template variables ({prompt}, {model}, {cwd}, {session}) are
// substituted at runtime. This lets you add new agent backends without writing
// any code.
package oneagent

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Backend defines how to invoke and parse output from an agent CLI.
// Populated by compiling a backendConfig from the JSON config file.
type Backend struct {
	Cmd          []string
	ResumeCmd    []string
	SystemPrompt string
	Format       string // "json" or "jsonl"
	Result       string
	ResultWhen   string
	ResultAppend bool
	Session      string
	SessionWhen  string
	Error        string
	ErrorWhen    string
	DefaultModel string
}

// Response is the normalized output from any backend.
type Response struct {
	Result   string `json:"result"`
	Session  string `json:"session"`
	ThreadID string `json:"thread_id,omitempty"`
	Backend  string `json:"backend"`
	Error    string `json:"error,omitempty"`
}

// StreamEvent is a normalized incremental event emitted during a streaming run.
type StreamEvent struct {
	Type     string `json:"type"`
	Backend  string `json:"backend"`
	ThreadID string `json:"thread_id,omitempty"`
	Session  string `json:"session,omitempty"`
	Delta    string `json:"delta,omitempty"`
	Result   string `json:"result,omitempty"`
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

// DefaultConfigPath returns the optional user override config path.
func DefaultConfigPath() string {
	return filepath.Join(ConfigDir(), "backends.json")
}

//go:embed defaults/backends.json
var embeddedBackends []byte

// LoadBackends loads embedded defaults when path is empty and merges the
// optional user override file (~/.config/oneagent/backends.json) on top.
// When path is non-empty, only that file is loaded.
func LoadBackends(path string) (map[string]Backend, error) {
	if path == "" {
		return loadDefaultBackends()
	}
	return loadBackendsFile(path)
}

func loadBackendsFile(path string) (map[string]Backend, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return loadCompactBackends(data)
}

func loadDefaultBackends() (map[string]Backend, error) {
	backends, err := loadCompactBackends(embeddedBackends)
	if err != nil {
		return nil, fmt.Errorf("invalid embedded backends: %w", err)
	}
	if err := mergeBackendsFile(backends, DefaultConfigPath()); err != nil {
		return nil, err
	}
	return backends, nil
}

func mergeBackendsFile(backends map[string]Backend, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	overrides, err := loadCompactBackends(data)
	if err != nil {
		return err
	}
	for name, backend := range overrides {
		backends[name] = backend
	}
	return nil
}

// Run executes a prompt against the specified backend and returns a normalized response.
func buildCmd(b Backend, opts RunOpts) (*exec.Cmd, error) {
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
	if len(args) == 0 {
		return nil, fmt.Errorf("backend %q produced an empty command", opts.Backend)
	}

	cmd := exec.Command(args[0], args[1:]...)
	if opts.CWD != "" && !containsVar(b.Cmd, "{cwd}") {
		cmd.Dir = opts.CWD
	}
	cmd.Env = os.Environ()
	return cmd, nil
}

// Run executes a prompt against the specified backend and returns a normalized response.
func Run(backends map[string]Backend, opts RunOpts) Response {
	return run(backends, opts, nil)
}

// RunStream executes a prompt and emits normalized streaming events as they arrive.
func RunStream(backends map[string]Backend, opts RunOpts, emit func(StreamEvent)) Response {
	resp := run(backends, opts, emit)
	emitFinal(emit, finalEvent(resp))
	return resp
}

func run(backends map[string]Backend, opts RunOpts, emit func(StreamEvent)) Response {
	b, ok := backends[opts.Backend]
	if !ok {
		return Response{Error: "backend not configured: " + opts.Backend, Backend: opts.Backend}
	}

	cmd, err := buildCmd(b, opts)
	if err != nil {
		return Response{Error: err.Error(), Backend: opts.Backend}
	}

	var result, session string

	switch b.Format {
	case "jsonl":
		result, session, err = runJSONL(cmd, b, opts.Backend, emit)
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

func runJSONL(cmd *exec.Cmd, b Backend, backend string, emit func(StreamEvent)) (result, session string, err error) {
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
	result, session, lastErr := scanJSONL(scanner, b, backend, emit)
	scanErr := scanner.Err()

	if err = cmd.Wait(); err != nil {
		if s := stderr.String(); s != "" {
			log.Printf("stderr: %s", strings.TrimSpace(s))
		}
		if result == "" && lastErr != "" {
			result = lastErr
		}
	}
	if err == nil && scanErr != nil {
		err = scanErr
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

func scanJSONL(scanner *bufio.Scanner, b Backend, backend string, emit func(StreamEvent)) (result, session, lastErr string) {
	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if v := extractField(line, b.ErrorWhen, b.Error); v != "" {
			lastErr = v
		}
		if v := extractField(line, b.SessionWhen, b.Session); v != "" {
			if v != session {
				session = v
				emitEvent(emit, StreamEvent{Type: "session", Backend: backend, Session: session})
			}
		}
		if v := extractField(line, b.ResultWhen, b.Result); v != "" {
			if b.ResultAppend {
				result += v
			} else {
				result = v
			}
			emitEvent(emit, StreamEvent{Type: "delta", Backend: backend, Session: session, Delta: v})
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
		if stringifyValue(jsonGet(m, k)) != v {
			return false
		}
	}
	return true
}

func stringifyValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

func finalEvent(resp Response) StreamEvent {
	event := StreamEvent{
		Backend:  resp.Backend,
		ThreadID: resp.ThreadID,
		Session:  resp.Session,
	}
	if resp.Error != "" {
		event.Type = "error"
		event.Error = resp.Error
		return event
	}
	event.Type = "done"
	event.Result = resp.Result
	return event
}

func emitEvent(emit func(StreamEvent), event StreamEvent) {
	if emit != nil {
		emit(event)
	}
}

func emitFinal(emit func(StreamEvent), event StreamEvent) {
	emitEvent(emit, event)
}
