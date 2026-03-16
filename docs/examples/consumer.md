# Consumer Example

This page describes a typical embedded consumer of `oneagent` as a Go library, such as a chat app, bot, TUI, editor integration, or service wrapper.

## What the Consumer Stores Itself

The consumer typically keeps its own app-level state:

- selected backend
- selected model
- selected thread ID
- UI-specific cursor or delivery state

That state lives in its own config directory and is separate from `oneagent`.

## What the Consumer Delegates to oneagent

For agent execution, the consumer delegates almost everything:

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

The consumer creates a placeholder or pending UI state, then:

1. logs `activity` events
2. accumulates `delta` text
3. updates the UI periodically
4. replaces the placeholder with `resp.Result` at the end

This is a good example of the intended library contract:

- `oneagent` owns normalization
- the consumer owns UI policy

## Why This Example Matters

This pattern is useful because it shows how little glue is needed once `oneagent` provides:

- a stable `RunOpts` input
- normalized `Response`
- normalized `StreamEvent`
- portable thread management

If you are embedding `oneagent` into a chat app, TUI, editor extension, or web service, this is the basic pattern to follow.
