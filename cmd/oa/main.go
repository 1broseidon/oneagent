package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/1broseidon/oneagent"
)

type cliOpts struct {
	backend    string
	model      string
	cwd        string
	session    string
	stream     bool
	thread     string
	configPath string
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
	}
	for i := 0; i < len(args); i++ {
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

	if args[0] == "list" {
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
	if len(o.prompt) == 0 {
		fmt.Fprintln(os.Stderr, "error: no prompt provided")
		os.Exit(1)
	}

	if o.thread != "" && o.session != "" {
		fmt.Fprintln(os.Stderr, "error: --thread and --session are mutually exclusive")
		os.Exit(1)
	}

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
		Backend:   backend,
		Prompt:    strings.Join(o.prompt, " "),
		Model:     o.model,
		CWD:       o.cwd,
		SessionID: o.session,
		ThreadID:  o.thread,
	}

	if o.stream {
		streamPrompt(backends, opts)
		return
	}

	var resp oneagent.Response
	if o.thread != "" {
		resp = oneagent.RunWithThread(backends, opts)
	} else {
		resp = oneagent.Run(backends, opts)
	}

	out, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(out))

	if resp.Error != "" {
		os.Exit(1)
	}
}

func streamPrompt(backends map[string]oneagent.Backend, opts oneagent.RunOpts) {
	emit := func(event oneagent.StreamEvent) {
		out, _ := json.Marshal(event)
		fmt.Println(string(out))
	}

	var resp oneagent.Response
	if opts.ThreadID != "" {
		resp = oneagent.RunWithThreadStream(backends, opts, emit)
	} else {
		resp = oneagent.RunStream(backends, opts, emit)
	}

	if resp.Error != "" {
		os.Exit(1)
	}
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
		model := b.DefaultModel
		if model == "" {
			model = "(default)"
		}
		fmt.Printf("%-12s %s format=%s model=%s\n", name, backendProgram(b), b.Format, model)
	}
}

func backendProgram(b oneagent.Backend) string {
	if len(b.Cmd) == 0 || b.Cmd[0] == "" {
		return "(invalid)"
	}
	return b.Cmd[0]
}

func usage() {
	fmt.Println(`oa - one agent, any backend

Usage:
  oa [flags] <prompt>
  oa list                        List configured backends
  oa thread list                 List threads
  oa thread show <id>            Show thread contents
  oa thread compact <id> [-b]    Summarize old turns

Flags:
  -b, --backend <name>           Backend to use (default: claude)
  -m, --model <model>            Model override
  -C, --cwd <dir>                Working directory
  -s, --session <id>             Resume session (mutually exclusive with -t)
  --stream                       Emit normalized JSONL events while running
  -t, --thread <id>              Start or continue a thread
  -c, --config <path>            Use only this config file

Output:
  Default: JSON with result, session, thread_id, backend, and error fields.
  --stream: JSONL events with session, activity, delta, and final done/error records.
Config:
  Built-in defaults: claude, codex, opencode, pi.
  ~/.config/oneagent/backends.json adds or replaces backends.
  See https://github.com/1broseidon/oneagent for format.`)
}
