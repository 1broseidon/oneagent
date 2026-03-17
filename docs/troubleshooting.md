# Troubleshooting

## Backend shows "(not installed)"

`oa list` checks whether each backend's CLI binary can be found. If it shows `(not installed)`, the binary isn't in your `$PATH` or any of the configured fallback paths.

### Fix 1: Add the CLI to your PATH

Find where the CLI is installed and add that directory to your shell profile:

```sh
# Find the binary
which claude        # or: find ~ -name claude -type f 2>/dev/null
```

Then add it to your PATH in `~/.zshrc`, `~/.bashrc`, or equivalent:

```sh
export PATH="$HOME/.claude/local:$PATH"
```

Restart your shell or run `source ~/.zshrc`.

### Fix 2: Add a paths entry to your backend config

If you don't want to modify your PATH, tell `oa` where to find the binary by adding a `paths` field to your backend config in `~/.config/oneagent/backends.json`:

```json
{
  "claude": {
    "paths": ["/opt/custom/bin", "~/.my-tools/bin"]
  }
}
```

Paths support `~` expansion. `oa` checks `$PATH` first, then searches these directories in order.

The embedded defaults already include common install locations for each backend. If your CLI is installed somewhere unusual, add it here.

### Common install locations

| Backend | Common paths |
|---------|-------------|
| Claude | `~/.claude/local`, `~/.local/bin`, `~/.npm-global/bin` |
| Codex | `~/.local/bin`, `~/.npm-global/bin` |
| OpenCode | `~/.opencode/bin`, `~/.local/bin` |
| Pi | `~/.local/bin`, `~/.npm-global/bin` |

## Backend command fails

If `oa -b <name>` returns an error like `"claude" not found in PATH`, the same fixes above apply. The error includes a link to this page.

If the CLI is found but the command fails, check that:

1. The CLI is signed in / authenticated
2. The CLI version supports the flags in the config (run the CLI directly to verify)
3. Your `~/.config/oneagent/backends.json` override isn't missing required fields (overrides replace the entire backend, not just the fields you specify)
