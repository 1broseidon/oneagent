#!/bin/bash
# converse.sh — multi-agent conversation
#
# Two agents discuss a problem, alternating turns on a shared thread.
# Each agent sees the full conversation history and builds on the other's response.
#
# Usage:
#   ./converse.sh "the service fails after a few minutes but I can't reproduce it"
#   ./converse.sh -a claude -b codex -t 3 "debug this memory leak"
#
# Options:
#   -a BACKEND   First agent (default: claude)
#   -b BACKEND   Second agent (default: codex)
#   -t TURNS     Number of round trips (default: 2)

AGENT_A="claude"
AGENT_B="codex"
TURNS=2

while getopts "a:b:t:" opt; do
  case $opt in
    a) AGENT_A="$OPTARG" ;;
    b) AGENT_B="$OPTARG" ;;
    t) TURNS="$OPTARG" ;;
  esac
done
shift $((OPTIND - 1))

PROMPT="$*"
if [ -z "$PROMPT" ]; then
  echo "usage: converse.sh [-a backend] [-b backend] [-t turns] <prompt>" >&2
  exit 1
fi

THREAD="converse-$$"
echo "Thread: $THREAD | $AGENT_A <-> $AGENT_B | $TURNS rounds"
echo "---"

# Initial prompt to agent A
echo "[$AGENT_A]"
RESPONSE=$(oa -b "$AGENT_A" -t "$THREAD" "$PROMPT")
echo "$RESPONSE"
echo "---"

# Alternating turns
for i in $(seq 1 "$TURNS"); do
  echo "[$AGENT_B] (round $i)"
  RESPONSE=$(oa -b "$AGENT_B" -t "$THREAD" "Continue the investigation. Build on what your colleague said. What would you check next?")
  echo "$RESPONSE"
  echo "---"

  if [ "$i" -lt "$TURNS" ]; then
    echo "[$AGENT_A] (round $i)"
    RESPONSE=$(oa -b "$AGENT_A" -t "$THREAD" "Continue the investigation. Build on what your colleague said. What would you check next?")
    echo "$RESPONSE"
    echo "---"
  fi
done

# Synthesis
echo "[synthesis - $AGENT_A]"
oa -b "$AGENT_A" -t "$THREAD" "Synthesize this discussion into a concise action plan. List the top findings and concrete next steps."
