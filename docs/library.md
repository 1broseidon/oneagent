# Library Guide

`oneagent` can be embedded directly from Go without shelling out to the `oa` CLI.

## Loading Backends

Use embedded defaults plus the optional user override file:

```go
backends, err := oneagent.LoadBackends("")
if err != nil {
	log.Fatal(err)
}

client := oneagent.Client{Backends: backends}
```

Use an explicit config file instead:

```go
backends, err := oneagent.LoadBackends("/path/to/backends.json")
if err != nil {
	log.Fatal(err)
}
```

## Final Responses

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

`Response` is normalized across backends:

```go
type Response struct {
	Result   string
	Session  string
	ThreadID string
	Backend  string
	Error    string
}
```

## Streaming

Use `RunStream` when you want incremental updates:

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

The normalized event model is intentionally small:

- `session`
- `activity`
- `delta`
- `done`
- `error`

## Portable Threads

Use `RunWithThread` or `RunWithThreadStream` to let `oneagent` own the conversation history:

```go
resp := client.RunWithThread(oneagent.RunOpts{
	Backend:  "codex",
	ThreadID: "auth-fix",
	Prompt:   "continue debugging",
	CWD:      "/path/to/project",
})
```

This gives you:

- native session reuse when the same backend is continuing the thread
- cross-backend replay when a different backend picks the thread up
- local thread storage and compaction helpers

Thread helpers:

```go
ids, err := client.ListThreads()
thread, err := client.LoadThread("auth-fix")
err = client.CompactThread("auth-fix", "claude")
```

## Custom Thread Storage

Consumers that need isolated thread state can inject a store:

```go
store := oneagent.FilesystemStore{Dir: "/tmp/my-app-threads"}
client := oneagent.Client{
	Backends: backends,
	Store:    store,
}
```

Package-level helpers still use the default filesystem-backed store for backward compatibility.

## Consumer Pattern

The most common embedding pattern is:

1. Keep app-level state such as selected backend, selected model, and current thread ID.
2. Call `RunWithThreadStream`.
3. Render `activity` and `delta` incrementally in your UI.
4. Replace or finalize the UI with `resp.Result` once the stream ends.

See [consumer.md](./examples/consumer.md) for a concrete example.
