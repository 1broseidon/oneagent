package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/1broseidon/oneagent"
)

type cliOpts struct {
	backend    string
	model      string
	cwd        string
	session    string
	json       bool
	text       bool
	stream     bool
	thread     string
	configPath string
	preRun     string
	postRun    string
	prompt     []string
}

func parseArgs(args []string) cliOpts {
	var o cliOpts
	flags := map[string]*string{
		"-b": &o.backend, "--backend": &o.backend,
		"-m": &o.model, "--model": &o.model,
		"-C": &o.cwd, "--cwd": &o.cwd,
		"-s": &o.session, "--session": &o.session,
		"-t": &o.thread, "--thread": &o.thread,
		"-c": &o.configPath, "--config": &o.configPath,
		"--pre-run":  &o.preRun,
		"--post-run": &o.postRun,
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			o.json = true
			continue
		case "--jsonl":
			o.json = true
			o.stream = true
			continue
		case "--text":
			o.text = true
			continue
		}
		if args[i] == "--stream" {
			o.stream = true
			continue
		}
		if dst, ok := flags[args[i]]; ok && i+1 < len(args) {
			*dst = args[i+1]
			i++
		} else {
			o.prompt = append(o.prompt, args[i])
		}
	}
	return o
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		usage()
		return
	}
	if args[0] == "-v" || args[0] == "--version" || args[0] == "version" {
		fmt.Printf("oa %s\n", buildVersion())
		return
	}

	if args[0] == "list" || args[0] == "backends" {
		args, configPath := parseConfigArgs(args[1:], "")
		if len(args) != 0 {
			fmt.Fprintln(os.Stderr, "usage: oa list [-c config]")
			os.Exit(1)
		}
		listBackends(configPath)
		return
	}

	if args[0] == "thread" {
		threadCmd(args[1:], "")
		return
	}

	o := parseArgs(args)
	runPrompt(o)
}

func runPrompt(o cliOpts) {
	o = readStdin(o)
	o = readEnvPrompt(o)
	validateRunPrompt(o)
	backends, opts := loadRunContext(o)
	dispatchPrompt(backends, opts, o)
}

// readStdin reads from stdin when it's a pipe and combines it with any
// positional prompt args. Pipe content becomes context, args become instructions.
func readStdin(o cliOpts) cliOpts {
	info, err := os.Stdin.Stat()
	if err != nil {
		return o
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return o // stdin is a terminal, not a pipe
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil || len(data) == 0 {
		return o
	}
	piped := strings.TrimSpace(string(data))
	if len(o.prompt) > 0 {
		// args are instructions, stdin is context
		o.prompt = []string{piped + "\n\n" + strings.Join(o.prompt, " ")}
	} else {
		o.prompt = []string{piped}
	}
	return o
}

// readEnvPrompt reads the prompt from OA_PROMPT when no prompt has been
// provided via args or stdin. The env var keeps the prompt out of ps output.
func readEnvPrompt(o cliOpts) cliOpts {
	if len(o.prompt) > 0 {
		return o
	}
	if v := os.Getenv("OA_PROMPT"); v != "" {
		o.prompt = []string{v}
	}
	return o
}

func validateRunPrompt(o cliOpts) {
	if len(o.prompt) == 0 {
		fmt.Fprintln(os.Stderr, "error: no prompt provided")
		os.Exit(1)
	}
	if o.thread != "" && o.session != "" {
		fmt.Fprintln(os.Stderr, "error: --thread and --session are mutually exclusive")
		os.Exit(1)
	}
	if o.text && o.json {
		fmt.Fprintln(os.Stderr, "error: --text and --json are mutually exclusive")
		os.Exit(1)
	}
}

func loadRunContext(o cliOpts) (map[string]oneagent.Backend, oneagent.RunOpts) {
	backends, err := oneagent.LoadBackends(o.configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	backend := o.backend
	if backend == "" {
		backend = "claude"
	}

	opts := oneagent.RunOpts{
		Backend:    backend,
		Prompt:     strings.Join(o.prompt, " "),
		Model:      o.model,
		CWD:        o.cwd,
		SessionID:  o.session,
		ThreadID:   o.thread,
		PreRunCmd:  o.preRun,
		PostRunCmd: o.postRun,
	}
	return backends, opts
}

func dispatchPrompt(backends map[string]oneagent.Backend, opts oneagent.RunOpts, o cliOpts) {
	if o.stream {
		streamPrompt(backends, opts, o.json)
		return
	}

	resp := oneagent.Run(backends, opts)
	if o.json {
		writeJSONResponse(resp)
		return
	}
	writeTextResponse(resp)
}

func streamPrompt(backends map[string]oneagent.Backend, opts oneagent.RunOpts, jsonOutput bool) {
	emit := func(event oneagent.StreamEvent) {
		if !jsonOutput {
			return
		}
		out, _ := json.Marshal(event)
		fmt.Println(string(out))
	}

	writer := textStreamWriter{}
	if !jsonOutput {
		emit = writer.Emit
	}

	resp := oneagent.RunStream(backends, opts, emit)

	if !jsonOutput {
		writer.Finish(resp)
		return
	}
	if resp.Error != "" {
		os.Exit(1)
	}
}

type textStreamWriter struct {
	wroteDelta       bool
	endedWithNewline bool
}

func (w *textStreamWriter) Emit(event oneagent.StreamEvent) {
	switch event.Type {
	case "activity":
		if event.Activity != "" {
			fmt.Fprintf(os.Stderr, "[activity] %s\n", event.Activity)
		}
	case "delta":
		fmt.Print(event.Delta)
		w.wroteDelta = true
		w.endedWithNewline = strings.HasSuffix(event.Delta, "\n")
	}
}

func (w *textStreamWriter) Finish(resp oneagent.Response) {
	if resp.Error != "" {
		if w.wroteDelta && !w.endedWithNewline {
			fmt.Fprintln(os.Stderr)
		}
		fmt.Fprintln(os.Stderr, resp.Error)
		os.Exit(1)
	}
	if !w.wroteDelta {
		fmt.Println(resp.Result)
		return
	}
	if !w.endedWithNewline {
		fmt.Println()
	}
}

func writeJSONResponse(resp oneagent.Response) {
	out, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(out))
	if resp.Error != "" {
		os.Exit(1)
	}
}

func writeTextResponse(resp oneagent.Response) {
	if resp.Error != "" {
		fmt.Fprintln(os.Stderr, resp.Error)
		os.Exit(1)
	}
	fmt.Println(resp.Result)
}

func threadCmd(args []string, configPath string) {
	args, configPath = parseConfigArgs(args, configPath)
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: oa thread <list|show|compact> [args]")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		ids, err := oneagent.ListThreads()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		for _, id := range ids {
			fmt.Println(id)
		}

	case "show":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: oa thread show <id>")
			os.Exit(1)
		}
		t, err := oneagent.LoadThread(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		out, _ := json.MarshalIndent(t, "", "  ")
		fmt.Println(string(out))

	case "compact":
		threadCompact(args[1:], configPath)

	default:
		fmt.Fprintf(os.Stderr, "unknown thread command: %s\n", args[0])
		os.Exit(1)
	}
}

func parseConfigArgs(args []string, configPath string) ([]string, string) {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "-c" || args[i] == "--config" {
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: missing value for --config")
				os.Exit(1)
			}
			configPath = args[i+1]
			i++
			continue
		}
		out = append(out, args[i])
	}
	return out, configPath
}

func threadCompact(args []string, configPath string) {
	backend := "claude"
	var threadID string

	for i := 0; i < len(args); i++ {
		if (args[i] == "-b" || args[i] == "--backend") && i+1 < len(args) {
			backend = args[i+1]
			i++
		} else {
			threadID = args[i]
		}
	}

	if threadID == "" {
		fmt.Fprintln(os.Stderr, "usage: oa thread compact <id> [-b backend]")
		os.Exit(1)
	}

	backends, err := oneagent.LoadBackends(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := oneagent.CompactThread(backends, threadID, backend); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("compacted")
}

func listBackends(configPath string) {
	backends, err := oneagent.LoadBackends(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	for name, b := range backends {
		_, found := oneagent.ResolveBackendProgram(b)
		model := b.DefaultModel
		if model == "" {
			model = ""
		}
		status := ""
		if !found {
			status = " (not installed)"
		}
		if model != "" {
			fmt.Printf("%-12s model=%s%s\n", name, model, status)
		} else {
			fmt.Printf("%-12s%s\n", name, status)
		}
	}
}

func usage() {
	fmt.Println(`oa - one agent, any backend

Usage:
  oa [flags] <prompt>
  oa version                     Show binary version
  oa list                        List configured backends
  oa thread list                 List threads
  oa thread show <id>            Show thread contents
  oa thread compact <id> [-b]    Summarize old turns

Flags:
  -b, --backend <name>           Backend to use (default: claude)
  -m, --model <model>            Model override
  -C, --cwd <dir>                Working directory
  -v, --version                  Show binary version
  --json                         Emit machine-readable JSON output
  --jsonl                        Alias for --stream --json
  --pre-run <cmd>                Run a shell command before backend execution; exit non-zero aborts
  --post-run <cmd>               Run a shell command after backend execution; result piped to stdin
  -s, --session <id>             Resume session (mutually exclusive with -t)
  --text                         Emit plain text output (default)
  --stream                       Stream live output while running
  -t, --thread <id>              Start or continue a thread
  -c, --config <path>            Use only this config file

Output:
  Default: plain text result for humans.
  --json: final JSON with result, session, thread_id, backend, and error fields.
  --stream: live text with activity lines and streamed assistant text.
  --stream --json: normalized JSONL events with session, activity, delta, and final done/error records.
  --jsonl: shortcut for --stream --json.
Config:
  Built-in defaults: claude, codex, opencode, pi.
  ~/.config/oneagent/backends.json adds or replaces backends.
  See https://github.com/1broseidon/oneagent for format.`)
}
