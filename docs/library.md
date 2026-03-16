# Go Library Guide

Use `oneagent` as a Go library to run AI agents directly from your application, without the `oa` CLI.

## Install

```sh
go get github.com/1broseidon/oneagent@latest
```

## Loading Backends

Load the embedded defaults plus any user overrides from `~/.config/oneagent/backends.json`:

```go
backends, err := oneagent.LoadBackends("")
if err != nil {
	log.Fatal(err)
}

client := oneagent.Client{Backends: backends}
```

Or load from an explicit config file instead:

```go
backends, err := oneagent.LoadBackends("/path/to/backends.json")
if err != nil {
	log.Fatal(err)
}
```

## Running a Prompt

For a single final response:

```go
resp := client.Run(oneagent.RunOpts{
	Backend: "claude",
	Prompt:  "explain this codebase",
	CWD:     "/path/to/project",
})

if resp.Error != "" {
	log.Fatal(resp.Error)
}

fmt.Println(resp.Result)
fmt.Println(resp.Session)
```

Every backend returns the same `Response` shape:

```go
type Response struct {
	Result   string // the agent's final answer
	Session  string // native session ID, for resuming later
	ThreadID string // portable thread ID, if using threads
	Backend  string // which backend produced this response
	Error    string // non-empty on failure
}
```

## Streaming

Use `RunStream` to receive incremental updates as the agent works:

```go
resp := client.RunStream(oneagent.RunOpts{
	Backend: "claude",
	Prompt:  "review the repo",
	CWD:     "/path/to/project",
}, func(ev oneagent.StreamEvent) {
	switch ev.Type {
	case "session":
		fmt.Println("session:", ev.Session)
	case "activity":
		fmt.Println("activity:", ev.Activity)
	case "delta":
		fmt.Print(ev.Delta)
	case "done":
		fmt.Println("\nfinal:", ev.Result)
	case "error":
		fmt.Println("\nerror:", ev.Error)
	}
})

if resp.Error != "" {
	log.Fatal(resp.Error)
}
```

The event types are intentionally small: `session`, `activity`, `delta`, `done`, and `error`.

## Portable Threads

Use `RunWithThread` or `RunWithThreadStream` to maintain conversation history across runs — even across different backends:

```go
resp := client.RunWithThread(oneagent.RunOpts{
	Backend:  "codex",
	ThreadID: "auth-fix",
	Prompt:   "continue debugging",
	CWD:      "/path/to/project",
})
```

This gives you:

- Native session reuse when continuing on the same backend
- Automatic context replay when switching to a different backend
- Local thread storage with compaction to keep long conversations manageable

Thread management:

```go
ids, err := client.ListThreads()
thread, err := client.LoadThread("auth-fix")
err = client.CompactThread("auth-fix", "claude")
```

## Custom Thread Storage

By default, threads are stored on disk at `~/.local/state/oneagent/threads/`. To store threads elsewhere (for example, in a database or isolated directory), inject a custom store:

```go
store := oneagent.FilesystemStore{Dir: "/tmp/my-app-threads"}
client := oneagent.Client{
	Backends: backends,
	Store:    store,
}
```

You can also implement the `Store` interface for fully custom storage:

```go
type Store interface {
	LoadThread(id string) (*Thread, error)
	SaveThread(thread *Thread) error
	ListThreads() ([]string, error)
}
```

## Typical Integration Pattern

Most applications follow this pattern:

1. Keep your own app-level state (selected backend, model, current thread ID).
2. Call `client.RunWithThreadStream` with the current state.
3. Render `activity` and `delta` events incrementally in your UI.
4. Finalize the UI with `resp.Result` once the stream ends.

See [examples/consumer.md](./examples/consumer.md) for a complete, runnable example.
