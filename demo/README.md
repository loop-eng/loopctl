# LoopCtl Demo & Test Harness

Real-time simulator for exercising LoopCtl's discovery, parsing, cost, and
spin-detection pipeline against synthetic Claude Code sessions — no real
agent session required.

## One-Command Trial

```bash
bash demo/trial.sh
```

Opens the real LoopCtl dashboard against five synthetic sessions running
concurrently — normal, spin-tool, spin-error, budget, and stall. Takes
~60 seconds; press `q` to quit (auto-cleans up demo data on exit).

## Quick Start (Manual)

```bash
# Build
go build -o bin/loopctl ./cmd/loopctl/
go build -o bin/loopctl-demo ./demo/

# Terminal 1: run a scenario (writes into ~/.claude/projects/-tmp-loopctl-demo/)
bin/loopctl-demo -scenario spin-tool

# Terminal 2: point the dashboard at it (also picks up your real sessions)
bin/loopctl --config demo/demo-config.yaml
```

## Scenarios

| Scenario     | What it does                                | Expected LoopCtl response |
|--------------|----------------------------------------------|----------------------------|
| `normal`     | Varied tool calls, file edits                 | Status stays Running -> Done, no alerts |
| `spin-tool`  | 8x identical Edit call                        | Status flips to SPIN after the 3rd identical call, alert in the Alerts panel |
| `spin-error` | 6x identical error result                     | Status flips to SPIN after the 3rd identical error, alert in the Alerts panel |
| `budget`     | 10 high-token iterations (opus-4-6 pricing)   | Budget warning then critical alert (needs `demo-config.yaml`'s lowered threshold — the $20/session default won't trip in this short a run) |
| `stall`      | One edit, then 10 reads, no further edits     | Stall warning after the gap since the last edit exceeds `spin.stall_minutes` (needs `demo-config.yaml`'s `stall_minutes: 1` — the 10-minute default won't trip in this short a run) |
| `all`        | Runs all five concurrently                    | The full dashboard experience — what `trial.sh` and `record.sh` use |

Pass `-speed <float>` to any scenario (default `1.0`) to scale how fast
events are written; `trial.sh`/`record.sh` run the `stall` scenario at
`0.8` (slower) so its real-time gap has room to clear the 60-second
threshold before the scenario's writer process exits.

## Demo Config

`demo/demo-config.yaml` lowers `budget.per_session_usd`,
`budget.warn_at_percent`, and `spin.stall_minutes` so the budget and stall
alerts can trigger within a ~60 second demo window instead of production
defaults ($20/session, 80%, 10 minutes). Point any manual run at it with
`--config demo/demo-config.yaml`.

## Recording

```bash
asciinema rec demo/loopctl-demo.cast --overwrite -c "bash demo/record.sh"
```

See the header comment in `demo/record.sh` for how the auto-quit works —
it spawns `loopctl` under `expect` and sends a real `q` keypress, since
sending SIGTERM/SIGINT to a process that isn't attached to a real
interactive terminal does not reliably exercise the same clean-quit path.

## Automated E2E Test Suite

```bash
bash demo/test_e2e.sh
```

Builds both binaries, generates all 5 scenarios at max speed, and validates
JSONL structure, session content, pipeline parsing (via `go test ./...`),
CLI flags (`--version`/`--help`), and `go vet`/`go build` cleanliness.

## Cleanup

```bash
bash demo/cleanup.sh
```

Removes the synthetic session directory, any `demo-*` exports under
`~/.config/loopctl/exports/`, and the `loopctl-demo` binary. Safe to run
any time. Pass `--all` to also remove `bin/loopctl`.
