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
	Source:   "my-app",  // tags turns with who produced them
})
```

This gives you:

- Native session reuse when continuing on the same backend
- Automatic context replay when switching to a different backend
- Local thread storage with compaction to keep long conversations manageable
- File locking — safe for multiple processes to share a thread concurrently
- Turn attribution — each turn records its `Source` so you can distinguish bot, cron, and user turns

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

## Hooks

Use `PreRun` and `PostRun` callbacks to run logic before and after agent execution:

```go
resp := client.Run(oneagent.RunOpts{
	Backend:  "claude",
	ThreadID: "daily-review",
	Prompt:   "summarize today",
	Source:   "cron-nightly",
	PreRun: func(opts *oneagent.RunOpts) error {
		// modify opts, set up worktree, validate environment
		opts.CWD = "/tmp/workspace"
		return nil // return error to abort
	},
	PostRun: func(ctx *oneagent.HookContext) {
		// notify, clean up, log
		fmt.Println("Done:", ctx.Response.Result)
	},
})
```

For shell-based hooks (useful from the CLI or config), use `PreRunCmd` and `PostRunCmd`:

```go
resp := client.Run(oneagent.RunOpts{
	Backend:    "claude",
	Prompt:     "summarize today",
	PostRunCmd: "curl -s -X POST https://hooks.example.com/notify -d @-",
})
```

Post-run shell hooks receive the result on stdin and environment variables: `OA_BACKEND`, `OA_THREAD_ID`, `OA_SOURCE`, `OA_MODEL`, `OA_CWD`, `OA_SESSION`, `OA_ERROR`, `OA_EXIT`. Pre-run hooks receive the same minus session/error/exit.

Hooks from the backend config (`pre_run`/`post_run` fields) and hooks from `RunOpts` both execute — config first, then per-invocation. Pre-run hooks abort the run on non-zero exit. Post-run hooks are best-effort.

## Typical Integration Pattern

Most applications follow this pattern:

1. Keep your own app-level state (selected backend, model, current thread ID).
2. Call `client.RunWithThreadStream` with the current state.
3. Render `activity` and `delta` events incrementally in your UI.
4. Finalize the UI with `resp.Result` once the stream ends.

See [examples/consumer.md](./examples/consumer.md) for a complete, runnable example.
