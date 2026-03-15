package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/1broseidon/oneagent"
)

type cliOpts struct {
	backend    string
	model      string
	cwd        string
	session    string
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
		"-c": &o.configPath, "--config": &o.configPath,
	}
	for i := 0; i < len(args); i++ {
		if dst, ok := flags[args[i]]; ok && i+1 < len(args) {
			*dst = args[i+1]
			i++
		} else {
			o.prompt = append(o.prompt, args[i])
		}
	}
	return o
}

func resolveConfig(path string) string {
	if path != "" {
		return path
	}
	return filepath.Join(oneagent.ConfigDir(), "backends.json")
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		usage()
		return
	}

	if args[0] == "list" {
		listBackends(resolveConfig(""))
		return
	}

	o := parseArgs(args)

	if len(o.prompt) == 0 {
		fmt.Fprintln(os.Stderr, "error: no prompt provided")
		os.Exit(1)
	}

	backends, err := oneagent.LoadBackends(resolveConfig(o.configPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	backend := o.backend
	if backend == "" {
		backend = "claude"
	}

	resp := oneagent.Run(backends, oneagent.RunOpts{
		Backend:   backend,
		Prompt:    strings.Join(o.prompt, " "),
		Model:     o.model,
		CWD:       o.cwd,
		SessionID: o.session,
	})

	out, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(out))

	if resp.Error != "" {
		os.Exit(1)
	}
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
		fmt.Printf("%-12s %s format=%s model=%s\n", name, b.Cmd[0], b.Format, model)
	}
}

func usage() {
	fmt.Println(`oa - one agent, any backend

Usage:
  oa [flags] <prompt>
  oa list                        List configured backends

Flags:
  -b, --backend <name>           Backend to use (default: claude)
  -m, --model <model>            Model override
  -C, --cwd <dir>                Working directory
  -s, --session <id>             Resume session
  -c, --config <path>            Config file (default: ~/.config/oneagent/backends.json)

Output:
  JSON with result, session, backend, and error fields.

Config:
  Define backends in ~/.config/oneagent/backends.json.
  See https://github.com/1broseidon/oneagent for format.`)
}
