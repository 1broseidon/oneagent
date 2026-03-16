# Consumer Example: tele

[`tele`](../../../tele/main.go) is a small Telegram bot that consumes `oneagent` as a Go library.

## What tele Stores Itself

`tele` keeps its own app-level state:

- selected backend
- selected model
- selected thread ID
- Telegram cursor position

That state lives in its own config directory and is separate from `oneagent`.

## What tele Delegates to oneagent

For agent execution, `tele` delegates almost everything:

- backend dispatch
- session handling
- portable threads
- streaming events
- final normalized result

The core dispatch path is:

```go
resp := oneagent.RunWithThreadStream(backends, opts, emit)
```

## UI Pattern

`tele` sends a placeholder Telegram message, then:

1. logs `activity` events
2. accumulates `delta` text
3. edits the Telegram message periodically
4. replaces the placeholder with `resp.Result` at the end

This is a good example of the intended library contract:

- `oneagent` owns normalization
- the consumer owns UI policy

## Why This Example Matters

`tele` is a useful reference for other consumers because it shows how little glue is needed once `oneagent` provides:

- a stable `RunOpts` input
- normalized `Response`
- normalized `StreamEvent`
- portable thread management

If you are embedding `oneagent` into a chat app, TUI, editor extension, or web service, this is the basic pattern to follow.
