#!/bin/bash
# Remove all LoopCtl demo artifacts: synthetic session data, exported files,
# and binaries built for testing/demoing. Safe to run repeatedly.

echo "Cleaning up LoopCtl demo artifacts..."

# Synthetic Claude Code session directory written by demo/main.go
rm -rf ~/.claude/projects/-tmp-loopctl-demo/

# Exports produced via the 'e' (export) keybinding while pointed at demo
# data (internal/app/model.go handleExport() writes to
# ~/.config/loopctl/exports/<sessionID>.json). Only remove exports whose
# session ID starts with "demo-" (the prefix every demo/main.go scenario
# uses, e.g. demo-normal-session-001) so a user's real exports are never
# touched by this script.
rm -f ~/.config/loopctl/exports/demo-*.json 2>/dev/null || true

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
rm -f "$REPO_DIR/bin/loopctl-demo"

# Scratch files used by trial.sh / record.sh
rm -f /tmp/loopctl-trial-sim.log /tmp/loopctl-record-sim.log

if [ "$1" = "--all" ]; then
  echo "  --all: also removing bin/loopctl"
  rm -f "$REPO_DIR/bin/loopctl"
fi

echo "Done."
