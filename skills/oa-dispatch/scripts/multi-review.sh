#!/bin/bash
# multi-review.sh — parallel reviews from multiple agents, merged into a summary
#
# Each agent reviews the same diff from a different angle, then a final
# agent synthesizes the reviews into one actionable summary.
#
# Usage:
#   ./multi-review.sh
#   ./multi-review.sh -s claude -d main

SYNTHESIZER="claude"
DIFF_BASE="main"

while getopts "s:d:" opt; do
  case $opt in
    s) SYNTHESIZER="$OPTARG" ;;
    d) DIFF_BASE="$OPTARG" ;;
  esac
done

DIFF=$(git diff "$DIFF_BASE")
if [ -z "$DIFF" ]; then
  echo "No diff against $DIFF_BASE"
  exit 0
fi

TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

echo "Reviewing diff against $DIFF_BASE..."

echo "$DIFF" | oa -b claude "Security review. Focus on injection, auth, data exposure." > "$TMPDIR/security.md" &
PID1=$!

echo "$DIFF" | oa -b codex "Performance review. Focus on efficiency, unnecessary allocations, hot paths." > "$TMPDIR/performance.md" &
PID2=$!

wait $PID1 $PID2

echo "--- Security Review ---"
cat "$TMPDIR/security.md"
echo ""
echo "--- Performance Review ---"
cat "$TMPDIR/performance.md"
echo ""
echo "--- Synthesis ---"
cat "$TMPDIR/security.md" "$TMPDIR/performance.md" | oa -b "$SYNTHESIZER" "Synthesize these two code reviews into one summary. List findings by severity, then actionable next steps."
