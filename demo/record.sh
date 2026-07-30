#!/bin/bash
# Records an asciinema cast of LoopCtl's live dashboard for the README.
# Usage: asciinema rec demo/loopctl-demo.cast --overwrite -c "bash demo/record.sh"
#    or: bash demo/record.sh   (to preview without recording)
#
# Requires `expect` to send a real 'q' keypress into loopctl's pty — sending
# SIGTERM/SIGINT to the process is NOT sufficient to reliably reproduce the
# same clean-quit path a real interactive Ctrl+C/q would hit inside a pty
# recording (see README Troubleshooting). If `expect` isn't available, this
# falls back to `timeout`, which forcibly kills the process and may leave
# the terminal in raw/alt-screen mode — run `reset` afterward if so.
set -e

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$REPO_DIR/bin"
DEMO_DIR="$HOME/.claude/projects/-tmp-loopctl-demo"
CONFIG="$REPO_DIR/demo/demo-config.yaml"
RECORD_SECONDS=45   # how long loopctl stays open before auto-quit

cleanup() {
  [ -n "${SIM_PID:-}" ] && kill $SIM_PID 2>/dev/null || true
  wait $SIM_PID 2>/dev/null || true
  rm -rf "$DEMO_DIR"
}
trap cleanup EXIT

cd "$REPO_DIR"
go build -o "$BIN/loopctl" ./cmd/loopctl/
go build -o "$BIN/loopctl-demo" ./demo/
rm -rf "$DEMO_DIR"

# Start synthetic sessions (staggered stall start — see demo/README.md).
"$BIN/loopctl-demo" -scenario stall      -speed 0.8 > /tmp/loopctl-record-sim.log 2>&1 &
SIM_PID="$!"
sleep 4
"$BIN/loopctl-demo" -scenario normal     -speed 1 >> /tmp/loopctl-record-sim.log 2>&1 & SIM_PID="$SIM_PID $!"
"$BIN/loopctl-demo" -scenario spin-tool  -speed 1 >> /tmp/loopctl-record-sim.log 2>&1 & SIM_PID="$SIM_PID $!"
"$BIN/loopctl-demo" -scenario spin-error -speed 1 >> /tmp/loopctl-record-sim.log 2>&1 & SIM_PID="$SIM_PID $!"
"$BIN/loopctl-demo" -scenario budget     -speed 1 >> /tmp/loopctl-record-sim.log 2>&1 & SIM_PID="$SIM_PID $!"

if command -v expect >/dev/null 2>&1; then
  # {$BIN/loopctl} / {$CONFIG} are Tcl brace-quoted so a path containing
  # spaces (e.g. this very repo, under ".../Loop Engineering/loopctl")
  # is passed to spawn as a single argument instead of being word-split.
  expect -c "
    set timeout $((RECORD_SECONDS + 10))
    spawn {$BIN/loopctl} --config {$CONFIG}
    sleep $RECORD_SECONDS
    send \"q\"
    expect eof
  "
else
  echo "expect not found — using forced-kill fallback (terminal may need 'reset' after)." >&2
  timeout -k 5 "$((RECORD_SECONDS + 3))s" "$BIN/loopctl" --config "$CONFIG" || true
fi
