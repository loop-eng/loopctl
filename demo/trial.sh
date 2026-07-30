#!/bin/bash
# LoopCtl Trial — interactive live-dashboard demo.
# Usage: bash demo/trial.sh
set -e

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$REPO_DIR/bin"
DEMO_DIR="$HOME/.claude/projects/-tmp-loopctl-demo"
CONFIG="$REPO_DIR/demo/demo-config.yaml"

bold() { printf "\033[1m%s\033[0m\n" "$1"; }
dim()  { printf "\033[2m%s\033[0m\n" "$1"; }
green(){ printf "\033[32m%s\033[0m\n" "$1"; }

cleanup() {
  [ -n "${SIM_PID:-}" ] && kill $SIM_PID 2>/dev/null || true
  wait $SIM_PID 2>/dev/null || true
  rm -rf "$DEMO_DIR"
  echo ""
  green "Demo data cleaned up. Thanks for trying LoopCtl."
}
trap cleanup EXIT

cd "$REPO_DIR"
echo "Building loopctl and demo simulator..."
go build -o "$BIN/loopctl" ./cmd/loopctl/
go build -o "$BIN/loopctl-demo" ./demo/

rm -rf "$DEMO_DIR"

bold "╔══════════════════════════════════════════════╗"
bold "║           LoopCtl — Interactive Trial         ║"
bold "╚══════════════════════════════════════════════╝"
echo ""
echo "  In a few seconds LoopCtl's live dashboard will open full-screen."
echo "  Five synthetic sessions will start and run over the next ~60s:"
echo ""
echo "    normal      — clean session, completes with no alerts"
echo "    spin-tool   — same Edit call repeated -> SPIN status + alert"
echo "    spin-error  — same error repeated -> SPIN status + alert"
echo "    budget      — cost climbs past the demo \$4 budget -> budget alert"
echo "    stall       — edits once, then only reads -> stall warning (~end of trial)"
echo ""
echo "  Watch the Sessions table (top) and Alerts panel (bottom-right)."
dim "  Press 'q' at any time to quit — this script cleans up after you."
echo ""
sleep 4

# Stagger the stall scenario ~4s early, at a slightly slower speed, so its
# edit-to-last-read gap (~51s at 1x) has the best chance of clearing the
# 60s stall_minutes:1 threshold in demo-config.yaml before the trial ends.
"$BIN/loopctl-demo" -scenario stall      -speed 0.8 > /tmp/loopctl-trial-sim.log 2>&1 &
SIM_PID="$!"
sleep 4
"$BIN/loopctl-demo" -scenario normal     -speed 1 >> /tmp/loopctl-trial-sim.log 2>&1 & SIM_PID="$SIM_PID $!"
"$BIN/loopctl-demo" -scenario spin-tool  -speed 1 >> /tmp/loopctl-trial-sim.log 2>&1 & SIM_PID="$SIM_PID $!"
"$BIN/loopctl-demo" -scenario spin-error -speed 1 >> /tmp/loopctl-trial-sim.log 2>&1 & SIM_PID="$SIM_PID $!"
"$BIN/loopctl-demo" -scenario budget     -speed 1 >> /tmp/loopctl-trial-sim.log 2>&1 & SIM_PID="$SIM_PID $!"

"$BIN/loopctl" --config "$CONFIG"
