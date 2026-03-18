#!/bin/bash
# daily-digest.sh — gather context, summarize, notify
#
# Collects git activity, open PRs, and CI status, sends to an agent
# for a daily standup summary, then pushes to Slack via webhook.
#
# Usage:
#   ./daily-digest.sh
#   SLACK_WEBHOOK=https://hooks.slack.com/... ./daily-digest.sh
#
# Designed for crontab:
#   0 9 * * * /path/to/daily-digest.sh

BACKEND="claude"
THREAD="daily-$(date +%Y%m%d)"

(
  echo "=== Git activity (last 24h) ==="
  git log --oneline --since="yesterday" 2>/dev/null || echo "(no git repo)"

  echo ""
  echo "=== Open PRs ==="
  gh pr list --state open 2>/dev/null || echo "(gh not available)"

  echo ""
  echo "=== Failing CI checks ==="
  gh run list --status failure --limit 5 2>/dev/null || echo "(gh not available)"
) | oa -b "$BACKEND" -t "$THREAD" \
  --post-run '
    if [ -n "$SLACK_WEBHOOK" ]; then
      curl -s -X POST "$SLACK_WEBHOOK" \
        -H "Content-Type: application/json" \
        -d "{\"text\": \"*Daily Digest*\n$(head -c 3000 | sed "s/\"/\\\\\"/g")\"}"
    fi
  ' \
  "Daily standup summary. What happened since yesterday? What needs attention today? Keep it concise."
