package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/george/oneagent"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		usage()
		return
	}

	backend := ""
	model := ""
	cwd := ""
	session := ""
	configPath := ""
	var prompt []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-b", "--backend":
			if i+1 < len(args) {
				backend = args[i+1]
				i++
			}
		case "-m", "--model":
			if i+1 < len(args) {
				model = args[i+1]
				i++
			}
		case "-C", "--cwd":
			if i+1 < len(args) {
				cwd = args[i+1]
				i++
			}
		case "-s", "--session":
			if i+1 < len(args) {
				session = args[i+1]
				i++
			}
		case "-c", "--config":
			if i+1 < len(args) {
				configPath = args[i+1]
				i++
			}
		case "list":
			listBackends(configPath)
			return
		default:
			prompt = append(prompt, args[i])
		}
	}

	if len(prompt) == 0 {
		fmt.Fprintln(os.Stderr, "error: no prompt provided")
		os.Exit(1)
	}

	if configPath == "" {
		configPath = filepath.Join(oneagent.ConfigDir(), "backends.json")
	}

	backends, err := oneagent.LoadBackends(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if backend == "" {
		backend = "claude"
	}

	resp := oneagent.Run(backends, oneagent.RunOpts{
		Backend:   backend,
		Prompt:    strings.Join(prompt, " "),
		Model:     model,
		CWD:       cwd,
		SessionID: session,
	})

	out, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(out))

	if resp.Error != "" {
		os.Exit(1)
	}
}

func listBackends(configPath string) {
	if configPath == "" {
		configPath = filepath.Join(oneagent.ConfigDir(), "backends.json")
	}
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
	fmt.Println(`oa — one agent, any backend

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
  See https://github.com/george/oneagent for format.`)
}
