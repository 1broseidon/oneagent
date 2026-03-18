#!/bin/bash
# self-heal.sh — run tests, fix failures, repeat
#
# Pipes test failures into an agent, runs tests again, repeats until
# green or max attempts reached.
#
# Usage:
#   ./self-heal.sh
#   ./self-heal.sh -b codex -n 5 -c "npm test"

BACKEND="codex"
MAX=3
CMD="go test ./..."

while getopts "b:n:c:" opt; do
  case $opt in
    b) BACKEND="$OPTARG" ;;
    n) MAX="$OPTARG" ;;
    c) CMD="$OPTARG" ;;
  esac
done

THREAD="heal-$$"

for i in $(seq 1 "$MAX"); do
  echo "--- attempt $i of $MAX ---"
  OUTPUT=$(eval "$CMD" 2>&1)
  if [ $? -eq 0 ]; then
    echo "Tests pass."
    exit 0
  fi
  echo "Tests failed. Sending to $BACKEND..."
  echo "$OUTPUT" | oa -b "$BACKEND" -t "$THREAD" "Fix these test failures. Do not ask for confirmation, just make the edits. Attempt $i of $MAX."
done

echo "Still failing after $MAX attempts."
exit 1
