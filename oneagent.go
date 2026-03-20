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
	"context"
	_ "embed"
	"encoding/json"
	"errors"
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
	Activity     string
	ActivityWhen string
	Delta        string
	DeltaWhen    string
	Result       string
	ResultWhen   string
	ResultAppend bool
	Session      string
	SessionWhen  string
	Error        string
	ErrorWhen    string
	DefaultModel string
	Paths        []string // additional directories to search for the CLI binary
	PromptStdin  bool     // pass prompt via stdin instead of argv
	PreRunCmd    string   // shell command to run before backend execution
	PostRunCmd   string   // shell command to run after backend execution
}

// Response is the normalized output from any backend.
type Response struct {
	Result   string `json:"result"`
	Session  string `json:"session"`
	ThreadID string `json:"thread_id,omitempty"`
	Backend  string `json:"backend"`
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

// StreamEvent is a normalized incremental event emitted during a streaming run.
type StreamEvent struct {
	Type     string `json:"type"`
	Backend  string `json:"backend"`
	ThreadID string `json:"thread_id,omitempty"`
	Session  string `json:"session,omitempty"`
	Activity string `json:"activity,omitempty"`
	Delta    string `json:"delta,omitempty"`
	Result   string `json:"result,omitempty"`
	Error    string `json:"error,omitempty"`
}

// HookContext is passed to the PostRun callback after a backend invocation completes.
type HookContext struct {
	Opts     RunOpts
	Response Response
}

// Client is an embeddable oneagent runtime with configurable backends and thread store.
type Client struct {
	Backends map[string]Backend
	Store    Store
}

// RunOpts configures a single agent invocation.
type RunOpts struct {
	Backend    string
	Prompt     string
	Model      string
	CWD        string
	SessionID  string
	ThreadID   string
	Source     string
	PreRun     func(*RunOpts) error // library callback: called before backend executes, can modify opts, return error to abort
	PostRun    func(*HookContext)   // library callback: called after response, for side effects
	PreRunCmd  string               // CLI shell command to run before backend execution
	PostRunCmd string               // CLI shell command to run after backend execution
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

// LoadOptions controls how backend configs are loaded.
type LoadOptions struct {
	// IncludeEmbedded loads the embedded default backends first.
	IncludeEmbedded bool
	// OverridePath is merged on top when IncludeEmbedded is true,
	// or loaded directly when IncludeEmbedded is false.
	OverridePath string
}

//go:embed defaults/backends.json
var embeddedBackends []byte

var (
	_ = (*Client).invokePrePhase
	_ = (*Client).runThreaded
	_ = runPreHook
	_ = (*Client).run
	_ = runJSON
	_ = runJSONL
)

// LoadBackends loads embedded defaults when path is empty and merges the
// optional user override file (~/.config/oneagent/backends.json) on top.
// When path is non-empty, only that file is loaded.
func LoadBackends(path string) (map[string]Backend, error) {
	if path == "" {
		return loadDefaultBackends()
	}
	return loadBackendsFile(path)
}

// LoadBackendsWithOptions loads backends using explicit options.
// This is useful for consumers that want the embedded defaults but
// need to own the override path instead of using ~/.config/oneagent/backends.json.
func LoadBackendsWithOptions(opts LoadOptions) (map[string]Backend, error) {
	if !opts.IncludeEmbedded {
		if opts.OverridePath == "" {
			return nil, fmt.Errorf("override path required when IncludeEmbedded is false")
		}
		return loadBackendsFile(opts.OverridePath)
	}

	backends, err := loadConfigBackends(embeddedBackends)
	if err != nil {
		return nil, fmt.Errorf("invalid embedded backends: %w", err)
	}
	if opts.OverridePath != "" {
		if err := mergeBackendsFile(backends, opts.OverridePath); err != nil {
			return nil, err
		}
	}
	return backends, nil
}

func loadBackendsFile(path string) (map[string]Backend, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return loadConfigBackends(data)
}

func loadDefaultBackends() (map[string]Backend, error) {
	backends, err := loadConfigBackends(embeddedBackends)
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
	overrides, err := loadConfigBackends(data)
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
	return buildCmdContext(context.Background(), b, opts)
}

func buildCmdContext(ctx context.Context, b Backend, opts RunOpts) (*exec.Cmd, error) {
	model := resolvedModel(b, opts)
	prompt := buildPrompt(b, opts)
	args, tmpl, err := buildCommandArgs(b, opts, prompt, model)
	if err != nil {
		return nil, err
	}

	cmd := commandForContext(ctx, resolveProgram(args[0], b.Paths), args[1:]...)
	configureCommand(cmd, b, opts, prompt, tmpl)
	return cmd, nil
}

func commandForContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	if ctx == nil {
		return exec.Command(name, args...)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	applyCancelableCommandContext(cmd, ctx)
	return cmd
}

func resolvedModel(b Backend, opts RunOpts) string {
	if opts.Model != "" {
		return opts.Model
	}
	return b.DefaultModel
}

func buildPrompt(b Backend, opts RunOpts) string {
	if opts.SessionID != "" || b.SystemPrompt == "" {
		return opts.Prompt
	}
	return b.SystemPrompt + "\n\n" + opts.Prompt
}

func buildCommandArgs(b Backend, opts RunOpts, prompt, model string) ([]string, []string, error) {
	tmpl := selectCommandTemplate(b, opts)
	args := substArgs(tmpl, map[string]string{
		"prompt":  promptArg(prompt, b.PromptStdin),
		"model":   model,
		"cwd":     opts.CWD,
		"session": opts.SessionID,
	})
	if len(args) == 0 {
		return nil, nil, fmt.Errorf("backend %q produced an empty command", opts.Backend)
	}
	return args, tmpl, nil
}

func selectCommandTemplate(b Backend, opts RunOpts) []string {
	if opts.SessionID != "" && len(b.ResumeCmd) > 0 {
		return b.ResumeCmd
	}
	return b.Cmd
}

func promptArg(prompt string, promptStdin bool) string {
	if promptStdin {
		return ""
	}
	return prompt
}

func configureCommand(cmd *exec.Cmd, b Backend, opts RunOpts, prompt string, tmpl []string) {
	if opts.CWD != "" && !containsVar(tmpl, "{cwd}") {
		cmd.Dir = opts.CWD
	}
	cmd.Env = os.Environ()
	if b.PromptStdin {
		cmd.Stdin = strings.NewReader(prompt)
	}
}

// Run executes a prompt against the specified backend and returns a normalized response.
func Run(backends map[string]Backend, opts RunOpts) Response {
	return RunContext(context.Background(), backends, opts)
}

// RunContext executes a prompt against the specified backend with cancellation support.
func RunContext(ctx context.Context, backends map[string]Backend, opts RunOpts) Response {
	return Client{Backends: backends}.RunContext(ctx, opts)
}

// RunStream executes a prompt and emits normalized streaming events as they arrive.
func RunStream(backends map[string]Backend, opts RunOpts, emit func(StreamEvent)) Response {
	return RunStreamContext(context.Background(), backends, opts, emit)
}

// RunStreamContext executes a prompt with cancellation support and emits normalized streaming events.
func RunStreamContext(ctx context.Context, backends map[string]Backend, opts RunOpts, emit func(StreamEvent)) Response {
	return Client{Backends: backends}.RunStreamContext(ctx, opts, emit)
}

// Run executes a prompt against the configured backends and returns a normalized response.
func (c Client) Run(opts RunOpts) Response {
	return c.RunContext(context.Background(), opts)
}

// RunContext executes a prompt against the configured backends and returns a normalized response.
func (c Client) RunContext(ctx context.Context, opts RunOpts) Response {
	return c.invokeContext(ctx, opts, nil)
}

// RunStream executes a prompt and emits normalized streaming events as they arrive.
func (c Client) RunStream(opts RunOpts, emit func(StreamEvent)) Response {
	return c.RunStreamContext(context.Background(), opts, emit)
}

// RunStreamContext executes a prompt and emits normalized streaming events as they arrive.
func (c Client) RunStreamContext(ctx context.Context, opts RunOpts, emit func(StreamEvent)) Response {
	return c.invokeContext(ctx, opts, emit)
}

// runDirect executes a prompt without hooks or terminal event emission.
// Used internally by CompactThread to avoid triggering hooks for internal calls.
func (c Client) runDirect(opts RunOpts) Response {
	return c.runContext(context.Background(), opts, nil)
}

// invoke is the full lifecycle wrapper for both threaded and non-threaded paths.
// Execution order:
//  1. Library PreRun callback (can modify RunOpts, error aborts)
//  2. Thread preparation (if ThreadID is set)
//  3. Config pre_run shell command (exit non-zero aborts)
//  4. CLI pre_run shell command (exit non-zero aborts)
//  5. Backend execution (run or runStream)
//  6. Thread persistence (if ThreadID is set)
//  7. Emit terminal stream event (done/error)
//  8. Config post_run shell command
//  9. CLI post_run shell command
//  10. Library PostRun callback
func (c Client) invoke(opts RunOpts, emit func(StreamEvent)) Response {
	return c.invokeContext(context.Background(), opts, emit)
}

func (c Client) invokeContext(ctx context.Context, opts RunOpts, emit func(StreamEvent)) Response {
	b, ok := c.Backends[opts.Backend]
	if !ok {
		resp := Response{Error: "backend not configured: " + opts.Backend, Backend: opts.Backend}
		emitFinal(emit, finalEvent(resp))
		return resp
	}

	// 1. Library PreRun callback, 2. Thread prep, 3-4. Shell pre-hooks
	thread, original, model, err := c.invokePrePhaseContext(ctx, &opts, b)
	if err != nil {
		resp := Response{Error: err.Error(), Backend: opts.Backend, ThreadID: opts.ThreadID}
		emitFinal(emit, finalEvent(resp))
		return resp
	}

	// 5. Backend execution (+ 6. thread persistence if threaded)
	var resp Response
	if thread != nil {
		resp = c.runThreadedContext(ctx, opts, emit, thread, original)
	} else {
		resp = c.runContext(ctx, opts, emit)
	}

	// 7. Emit terminal stream event
	emitFinal(emit, finalEvent(resp))

	// 8-10. Post-run hooks and callback
	c.invokePostPhase(opts, b, model, original, resp)

	return resp
}

// invokePrePhase runs lifecycle steps 1-4: library callback, thread prep, shell pre-hooks.
// Returns the thread (nil if non-threaded), original prompt, resolved model, and any abort error.
func (c Client) invokePrePhase(opts *RunOpts, b Backend) (*Thread, string, string, error) {
	return c.invokePrePhaseContext(context.Background(), opts, b)
}

func (c Client) invokePrePhaseContext(ctx context.Context, opts *RunOpts, b Backend) (*Thread, string, string, error) {
	// 1. Library PreRun callback
	if opts.PreRun != nil {
		if err := opts.PreRun(opts); err != nil {
			return nil, "", "", fmt.Errorf("pre-run callback: %w", err)
		}
	}

	// 2. Thread preparation
	var thread *Thread
	original := opts.Prompt
	if opts.ThreadID != "" {
		var err error
		thread, err = c.threadStore().LoadThread(opts.ThreadID)
		if err != nil {
			return nil, "", "", err
		}
		prepareThreadPrompt(thread, opts)
	}

	model := opts.Model
	if model == "" {
		model = b.DefaultModel
	}

	// 3. Config pre_run shell command
	if b.PreRunCmd != "" {
		if err := runPreHookContext(ctx, b.PreRunCmd, hookEnvPre(*opts, model)); err != nil {
			return nil, "", "", fmt.Errorf("config pre_run hook: %w", err)
		}
	}

	// 4. CLI pre_run shell command
	if opts.PreRunCmd != "" {
		if err := runPreHookContext(ctx, opts.PreRunCmd, hookEnvPre(*opts, model)); err != nil {
			return nil, "", "", fmt.Errorf("cli pre_run hook: %w", err)
		}
	}

	return thread, original, model, nil
}

// invokePostPhase runs lifecycle steps 8-10: config post-hook, CLI post-hook, library callback.
func (c Client) invokePostPhase(opts RunOpts, b Backend, model, originalPrompt string, resp Response) {
	// 8. Config post_run shell command
	if b.PostRunCmd != "" {
		runPostHook(b.PostRunCmd, hookEnvPost(opts, model, resp), resp.Result)
	}

	// 9. CLI post_run shell command
	if opts.PostRunCmd != "" {
		runPostHook(opts.PostRunCmd, hookEnvPost(opts, model, resp), resp.Result)
	}

	// 10. Library PostRun callback
	if opts.PostRun != nil {
		hookOpts := opts
		hookOpts.Prompt = originalPrompt
		opts.PostRun(&HookContext{Opts: hookOpts, Response: resp})
	}
}

// runThreaded handles thread preparation and persistence around the core run.
// Steps 2 (load) and 6 (persist) from the invoke lifecycle.
func (c Client) runThreaded(opts RunOpts, emit func(StreamEvent), thread *Thread, original string) Response {
	return c.runThreadedContext(context.Background(), opts, emit, thread, original)
}

func (c Client) runThreadedContext(ctx context.Context, opts RunOpts, emit func(StreamEvent), thread *Thread, original string) Response {
	var streamSaveErr error
	resp := c.runContext(ctx, opts, func(event StreamEvent) {
		event.ThreadID = opts.ThreadID
		if event.Type == "session" && event.Session != "" {
			thread.NativeSessions[opts.Backend] = event.Session
			if err := c.threadStore().SaveThread(thread); err != nil && streamSaveErr == nil {
				streamSaveErr = err
			}
		}
		emitEvent(emit, event)
	})
	if streamSaveErr != nil {
		resp.Error = "thread save failed: " + streamSaveErr.Error()
	}

	// 6. Thread persistence
	thread.recordTurns(original, resp, opts.Source)
	if err := c.threadStore().SaveThread(thread); err != nil {
		resp.Error = "thread save failed: " + err.Error()
	}
	resp.ThreadID = opts.ThreadID
	return resp
}

// hookEnvPre builds environment variables for pre-run shell hooks.
func hookEnvPre(opts RunOpts, model string) []string {
	env := os.Environ()
	env = append(env,
		"OA_BACKEND="+opts.Backend,
		"OA_MODEL="+model,
	)
	if opts.ThreadID != "" {
		env = append(env, "OA_THREAD_ID="+opts.ThreadID)
	}
	if opts.Source != "" {
		env = append(env, "OA_SOURCE="+opts.Source)
	}
	if opts.CWD != "" {
		env = append(env, "OA_CWD="+opts.CWD)
	}
	return env
}

// hookEnvPost builds environment variables for post-run shell hooks.
func hookEnvPost(opts RunOpts, model string, resp Response) []string {
	env := hookEnvPre(opts, model)
	env = append(env, "OA_SESSION="+resp.Session)
	exitCode := "0"
	errMsg := ""
	if resp.Error != "" {
		exitCode = "1"
		errMsg = resp.Error
	}
	env = append(env,
		"OA_ERROR="+errMsg,
		"OA_EXIT="+exitCode,
	)
	return env
}

// runPreHook executes a pre-run shell command via sh -c. Non-zero exit aborts the run.
func runPreHook(cmdStr string, env []string) error {
	return runPreHookContext(context.Background(), cmdStr, env)
}

func runPreHookContext(ctx context.Context, cmdStr string, env []string) error {
	cmd := commandForContext(ctx, "sh", "-c", cmdStr)
	cmd.Env = env
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if s := strings.TrimSpace(stderr.String()); s != "" {
			return fmt.Errorf("%s: %w", s, err)
		}
		return err
	}
	return nil
}

// runPostHook executes a post-run shell command via sh -c. Errors are logged but do not
// affect the response (best-effort).
func runPostHook(cmdStr string, env []string, result string) {
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdin = strings.NewReader(result)
	cmd.Env = env
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if s := strings.TrimSpace(stderr.String()); s != "" {
			log.Printf("post-run hook failed: %v: %s", err, s)
			return
		}
		log.Printf("post-run hook failed: %v", err)
	}
}

func (c Client) run(opts RunOpts, emit func(StreamEvent)) Response {
	return c.runContext(context.Background(), opts, emit)
}

type execMeta struct {
	ExitCode int
	Stderr   string
}

func (c Client) runContext(ctx context.Context, opts RunOpts, emit func(StreamEvent)) Response {
	b, ok := c.Backends[opts.Backend]
	if !ok {
		return Response{Error: "backend not configured: " + opts.Backend, Backend: opts.Backend}
	}

	cmd, err := buildCmdContext(ctx, b, opts)
	if err != nil {
		return Response{Error: err.Error(), Backend: opts.Backend}
	}

	var result, session string
	var meta execMeta

	switch b.Format {
	case "jsonl":
		result, session, meta, err = runJSONLWithMeta(cmd, b, opts.Backend, emit)
	default:
		result, session, meta, err = runJSONWithMeta(cmd, b)
	}

	if session == "" && opts.SessionID != "" {
		session = opts.SessionID
	}

	resp := Response{
		Result:   result,
		Session:  session,
		Backend:  opts.Backend,
		ExitCode: meta.ExitCode,
		Stderr:   meta.Stderr,
	}
	populateExecMeta(&resp, err)
	if err != nil {
		log.Printf("%s error: %v", opts.Backend, err)
		if errors.Is(err, exec.ErrNotFound) {
			resp.Error = fmt.Sprintf("%q not found in PATH. Is %s installed? See https://github.com/1broseidon/oneagent/blob/main/docs/troubleshooting.md", cmd.Path, opts.Backend)
		} else if ctxErr := contextError(ctx, err); ctxErr != nil {
			resp.Error = ctxErr.Error()
		} else if result != "" {
			resp.Error = result
		} else {
			resp.Error = err.Error()
		}
		return resp
	}

	return resp
}

func contextError(ctx context.Context, err error) error {
	if ctx == nil || err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return nil
}

func populateExecMeta(resp *Response, err error) {
	if err == nil {
		return
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return
	}
	if resp.ExitCode == 0 {
		resp.ExitCode = exitErr.ExitCode()
	}
	if resp.Stderr == "" && len(exitErr.Stderr) > 0 {
		resp.Stderr = string(exitErr.Stderr)
	}
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

// resolveProgram finds a program binary. It checks $PATH first, then falls back
// to the extra directories listed in paths. Tilde (~) is expanded to $HOME.
// Returns the original name if nothing is found, letting exec.Command produce
// the standard "not found" error.
func resolveProgram(name string, paths []string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	for _, dir := range paths {
		if strings.HasPrefix(dir, "~/") && home != "" {
			dir = filepath.Join(home, dir[2:])
		}
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return name
}

// ResolveBackendProgram returns the resolved path for a backend's CLI binary,
// checking $PATH first, then the backend's configured paths.
// Returns the program name and whether it was found.
func ResolveBackendProgram(b Backend) (string, bool) {
	if len(b.Cmd) == 0 || b.Cmd[0] == "" {
		return "(invalid)", false
	}
	resolved := resolveProgram(b.Cmd[0], b.Paths)
	if filepath.IsAbs(resolved) {
		return resolved, true
	}
	// resolveProgram returned the bare name — not found
	return b.Cmd[0], false
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
	result, session, _, err = runJSONWithMeta(cmd, b)
	return result, session, err
}

func runJSONWithMeta(cmd *exec.Cmd, b Backend) (result, session string, meta execMeta, err error) {
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.Printf("stderr: %s", exitErr.Stderr)
			meta.ExitCode = exitErr.ExitCode()
			meta.Stderr = string(exitErr.Stderr)
		}
		return "", "", meta, err
	}

	var blob map[string]any
	if err := json.Unmarshal(out, &blob); err != nil {
		return strings.TrimSpace(string(out)), "", meta, nil
	}

	if b.ErrorWhen != "" && matchWhen(blob, b.ErrorWhen) {
		errMsg, _ := jsonGet(blob, b.Error).(string)
		if errMsg != "" {
			return errMsg, "", meta, fmt.Errorf("%s", errMsg)
		}
	}
	result, _ = jsonGet(blob, b.Result).(string)
	session, _ = jsonGet(blob, b.Session).(string)
	return result, session, meta, nil
}

func runJSONL(cmd *exec.Cmd, b Backend, backend string, emit func(StreamEvent)) (result, session string, err error) {
	result, session, _, err = runJSONLWithMeta(cmd, b, backend, emit)
	return result, session, err
}

func runJSONLWithMeta(cmd *exec.Cmd, b Backend, backend string, emit func(StreamEvent)) (result, session string, meta execMeta, err error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", meta, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", "", meta, err
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	result, session, lastErr := scanJSONL(scanner, b, backend, emit)
	scanErr := scanner.Err()

	if err = cmd.Wait(); err != nil {
		meta.Stderr = stderr.String()
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			meta.ExitCode = exitErr.ExitCode()
		}
		if s := stderr.String(); s != "" {
			log.Printf("stderr: %s", strings.TrimSpace(s))
		}
		if result == "" && lastErr != "" {
			result = lastErr
		}
	} else if stderr.Len() > 0 {
		meta.Stderr = stderr.String()
	}
	if scanErr != nil {
		log.Printf("scanner error: %v", scanErr)
		if err != nil {
			err = errors.Join(err, scanErr)
		} else {
			err = scanErr
		}
	}
	return result, session, meta, err
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
	deltaField := b.Delta
	deltaWhen := b.DeltaWhen
	if deltaField == "" {
		deltaField = b.Result
		deltaWhen = b.ResultWhen
	}

	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		processJSONLLine(line, b, backend, emit, deltaField, deltaWhen, &result, &session, &lastErr)
	}
	return
}

func processJSONLLine(line map[string]any, b Backend, backend string, emit func(StreamEvent), deltaField, deltaWhen string, result, session, lastErr *string) {
	if v := extractField(line, b.ErrorWhen, b.Error); v != "" {
		*lastErr = v
	}
	if v := extractField(line, b.SessionWhen, b.Session); v != "" {
		emitSession(emit, backend, session, v)
	}
	if v := extractField(line, b.ResultWhen, b.Result); v != "" {
		appendResult(result, v, b.ResultAppend)
	}
	if v := extractTemplate(line, b.ActivityWhen, b.Activity); v != "" {
		emitEvent(emit, StreamEvent{Type: "activity", Backend: backend, Session: *session, Activity: v})
	}
	if v := extractField(line, deltaWhen, deltaField); v != "" {
		emitEvent(emit, StreamEvent{Type: "delta", Backend: backend, Session: *session, Delta: v})
	}
}

func emitSession(emit func(StreamEvent), backend string, current *string, next string) {
	if next == *current {
		return
	}
	*current = next
	emitEvent(emit, StreamEvent{Type: "session", Backend: backend, Session: next})
}

func appendResult(result *string, next string, appendMode bool) {
	if appendMode {
		*result += next
		return
	}
	*result = next
}

// jsonGet walks a dot-separated path into a map.
func jsonGet(m map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var cur any = m
	for _, p := range parts {
		if obj, ok := cur.(map[string]any); ok {
			cur = obj[p]
			continue
		}
		if arr, ok := cur.([]any); ok {
			if len(arr) == 0 {
				return nil
			}
			var idx int
			if _, err := fmt.Sscanf(p, "%d", &idx); err != nil {
				return nil
			}
			if idx < 0 {
				idx = len(arr) + idx
			}
			if idx < 0 || idx >= len(arr) {
				return nil
			}
			cur = arr[idx]
			continue
		}
		return nil
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

func extractTemplate(line map[string]any, when, tmpl string) string {
	if tmpl == "" || when == "" || !matchWhen(line, when) {
		return ""
	}
	if !strings.Contains(tmpl, "{") {
		return strings.TrimSpace(stringifyValue(jsonGet(line, tmpl)))
	}

	var out strings.Builder
	rest := tmpl
	for {
		open := strings.IndexByte(rest, '{')
		if open < 0 {
			out.WriteString(rest)
			break
		}
		out.WriteString(rest[:open])
		rest = rest[open+1:]

		close := strings.IndexByte(rest, '}')
		if close < 0 {
			out.WriteByte('{')
			out.WriteString(rest)
			break
		}
		out.WriteString(stringifyValue(jsonGet(line, rest[:close])))
		rest = rest[close+1:]
	}
	return strings.Join(strings.Fields(out.String()), " ")
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
