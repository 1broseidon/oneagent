# Example Scripts

These scripts demonstrate composable patterns using `oa`'s four primitives: pipes, chains, hooks, and threads. Each is a standalone shell script — copy, modify, use.

| Script | What it does |
|--------|-------------|
| [converse.sh](./converse.sh) | Two agents discuss a problem, alternating turns on a shared thread, then synthesize findings |
| [self-heal.sh](./self-heal.sh) | Run tests, pipe failures to an agent for fixing, repeat until green |
| [multi-review.sh](./multi-review.sh) | Parallel code reviews from different agents, merged into one summary |
| [daily-digest.sh](./daily-digest.sh) | Gather git/PR/CI context, summarize, push to Slack — designed for crontab |

## Quick start

```sh
chmod +x examples/*.sh

# Two agents debug a problem together
./examples/converse.sh "the service crashes after 5 minutes under load"

# Auto-fix failing tests (up to 3 attempts)
./examples/self-heal.sh

# Parallel security + performance review of your branch
./examples/multi-review.sh

# Daily digest to Slack
SLACK_WEBHOOK=https://hooks.slack.com/... ./examples/daily-digest.sh
```

## Writing your own

These scripts use standard shell — no special framework. The building blocks:

- `oa -b backend "prompt"` — run an agent
- `content | oa -b backend "instruction"` — pipe context in
- `oa -t thread "step 1" && oa -t thread "step 2"` — chain with shared memory
- `--pre-run` / `--post-run` — lifecycle hooks with env vars
- `$OA_BACKEND`, `$OA_THREAD_ID`, `$OA_EXIT`, etc. — available in hooks
