#!/usr/bin/env bash
# Demo script to simulate inter-agent messaging activity
# Usage: ./scripts/demo-messages.sh [--fast] [--count N]

set -euo pipefail

INTERVAL=2
COUNT=0  # 0 = infinite

while [[ $# -gt 0 ]]; do
    case $1 in
        --fast) INTERVAL=0.5; shift ;;
        --count) COUNT=$2; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# Agent names for simulation
AGENTS=("agent-alpha" "agent-beta" "agent-gamma" "agent-delta")

# Topics for different message types
TOPICS=(
    "build.started"
    "build.complete"
    "test.results"
    "deploy.status"
    "task.assigned"
    "task.complete"
    "error.reported"
    "status.update"
)

# Sample messages
MESSAGES=(
    "Starting build process..."
    "Build completed successfully"
    "All tests passing (42/42)"
    "Deployment to staging complete"
    "Picked up task: implement feature X"
    "Task completed, ready for review"
    "Error encountered in module Y"
    "Status: working on authentication"
    "Analyzing codebase structure"
    "Found 3 potential improvements"
    "Refactoring complete"
    "PR ready for review"
    "Investigating test failure"
    "Root cause identified"
    "Fix implemented and verified"
)

echo "Starting demo message simulation..."
echo "Press Ctrl+C to stop"
echo ""

i=0
while true; do
    # Pick random agent, topic, and message
    agent=${AGENTS[$RANDOM % ${#AGENTS[@]}]}
    topic=${TOPICS[$RANDOM % ${#TOPICS[@]}]}
    message=${MESSAGES[$RANDOM % ${#MESSAGES[@]}]}

    # Publish the message
    echo "[$agent] -> $topic: $message"
    hive msg pub -t "$topic" -s "$agent" "$message" 2>/dev/null || true

    # Also simulate a subscribe from another agent occasionally
    if (( RANDOM % 3 == 0 )); then
        subscriber=${AGENTS[$RANDOM % ${#AGENTS[@]}]}
        sub_topic=${TOPICS[$RANDOM % ${#TOPICS[@]}]}
        echo "[$subscriber] <- subscribed to $sub_topic"
        hive msg sub -t "$sub_topic" -n 1 >/dev/null 2>&1 || true
    fi

    i=$((i + 1))

    # Check count limit
    if [[ $COUNT -gt 0 && $i -ge $COUNT ]]; then
        echo ""
        echo "Sent $COUNT messages, stopping."
        break
    fi

    sleep "$INTERVAL"
done
