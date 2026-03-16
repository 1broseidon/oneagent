# Building an App with oneagent

This example shows a complete, runnable Go program that uses `oneagent` as a library. It demonstrates the typical integration pattern: load backends, stream a prompt, and render output incrementally.

## Full Example

```go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/1broseidon/oneagent"
)

func main() {
	// Load embedded backend defaults + any user overrides.
	backends, err := oneagent.LoadBackends("")
	if err != nil {
		log.Fatal(err)
	}

	client := oneagent.Client{Backends: backends}

	// Build the request. ThreadID makes the conversation portable
	// across backends — omit it for one-shot prompts.
	opts := oneagent.RunOpts{
		Backend:  "claude",
		ThreadID: "my-thread",
		Prompt:   "explain the main function",
		CWD:      ".",
	}

	// Stream the response, rendering events as they arrive.
	resp := client.RunWithThreadStream(opts, func(ev oneagent.StreamEvent) {
		switch ev.Type {
		case "session":
			fmt.Fprintf(os.Stderr, "session: %s\n", ev.Session)
		case "activity":
			fmt.Fprintf(os.Stderr, "[%s]\n", ev.Activity)
		case "delta":
			fmt.Print(ev.Delta)
		case "done":
			// Stream finished — resp.Result has the full text.
		case "error":
			fmt.Fprintf(os.Stderr, "error: %s\n", ev.Error)
		}
	})

	if resp.Error != "" {
		log.Fatal(resp.Error)
	}

	fmt.Fprintf(os.Stderr, "\nthread: %s  session: %s\n", resp.ThreadID, resp.Session)
}
```

## What This Demonstrates

**Backend loading** — `LoadBackends("")` loads the built-in defaults (Claude, Codex, OpenCode, Pi) and merges any overrides from `~/.config/oneagent/backends.json`.

**Streaming** — `RunWithThreadStream` calls the backend CLI and emits normalized events as they arrive. Your app renders `activity` and `delta` events immediately instead of waiting for the full response.

**Portable threads** — Setting `ThreadID` tells oneagent to maintain conversation history. If you later switch to a different backend (e.g., `codex`), oneagent replays the thread context automatically.

**Normalized output** — Every backend returns the same `Response` and `StreamEvent` types, so your rendering code works regardless of which agent is running.

## Adapting This Pattern

- **Chat app or TUI**: Replace `fmt.Print` with your UI rendering logic. Use `activity` events for status indicators and `delta` events for incremental text display.
- **Web service**: Marshal `StreamEvent` to JSON and send as server-sent events (SSE) or WebSocket frames.
- **Editor extension**: Use `activity` events to show progress in a status bar and `delta` events to populate an output panel.
- **One-shot scripts**: Drop `ThreadID` and use `client.Run` instead of `RunWithThreadStream` for a simple prompt-in, result-out flow.
