# LoopCtl

[![CI](https://github.com/loop-eng/loopctl/actions/workflows/ci.yaml/badge.svg)](https://github.com/loop-eng/loopctl/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/loop-eng/loopctl)](https://goreportcard.com/report/github.com/loop-eng/loopctl)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**htop for AI coding agents** — a live terminal dashboard that monitors all your Claude Code and Codex CLI sessions with real-time cost tracking, context health, and spin detection.

```
┌─────────────────────── Sessions Table ───────────────────────────────────┐
│ Status │ Project        │ Model            │ Duration │ Cost  │ Context │
│ ● Run  │ my-api         │ claude-opus-4-6  │ 12:34    │ $4.52 │ 45%     │
│ ● Run  │ frontend       │ claude-sonnet-4-5│ 3:21     │ $1.20 │ 22%     │
│ ⊘ SPIN │ data-pipeline  │ claude-opus-4-6  │ 8:45     │ $12.8 │ 89%     │
│ ○ Done │ cli-tool       │ o4-mini          │ 0:45     │ $0.30 │ 15%     │
├─────────┬──────────────┬────────────────────────────────────────────────┤
│  Cost   │   Context    │                  Alerts                        │
│         │              │                                                │
│ Ses $4.52│ Fill 45%    │ ✗ data-pipeline: same tool call repeated 5x   │
│ Rate $0.3│ ████░░░░    │ ⚠ data-pipeline: budget 89%                   │
│ Today $18│ Compact 2   │                                                │
├─────────┴──────────────┴────────────────────────────────────────────────┤
│ [q]uit [K]ill [e]xport [?]help [tab]panel [enter]detail [/]filter [h]ist │
└──────────────────────────────────────────────────────────────────────────┘
```

## The Problem

You're running 3 Claude Code sessions and a Codex task simultaneously. Right now you have no way to see:

- **How much each session is costing** in real-time
- **Whether any session is spinning** (repeating the same tool call, echoing errors)
- **How full the context window is** (approaching compaction? cache hit rate tanking?)
- **Which session to kill** when costs spike

LoopCtl gives you one terminal command to see everything.

## Install

```bash
# Homebrew (macOS/Linux)
brew install loop-eng/tap/loopctl

# Go install
go install github.com/loop-eng/loopctl/cmd/loopctl@latest

# Binary releases
# https://github.com/loop-eng/loopctl/releases
```

## Try It

No real agent session handy? Run the bundled simulator:

```bash
bash demo/trial.sh
```

Opens the real LoopCtl dashboard against five synthetic sessions running
concurrently — normal, spin-tool, spin-error, budget, and stall alerts all
trip within about a minute. Press `q` to quit; cleans up after itself. See
[demo/README.md](demo/README.md) for manual scenario control and recording
your own asciinema cast.

## Quick Start

```bash
# Just run it — zero config, auto-discovers sessions
loopctl

# With debug logging
loopctl --verbose

# Custom config
loopctl --config ~/.config/loopctl/config.yaml
```

LoopCtl auto-discovers Claude Code sessions from `~/.claude/projects/` and Codex CLI sessions from `~/.codex/sessions/`. No setup required.

**Supported sources today:** Claude Code, Codex CLI, plus [LTF](#ltf-trace-integration) trace enrichment for Claude Code sessions.
**Gemini CLI**: planned, tracked in [#7](https://github.com/loop-eng/loopctl/issues/7) — the on-disk format needs a live verification pass before a parser can be built against it.
**Cursor**: not supported. Cursor's local session storage is SQLite-based with no documented, stable per-session format — contributions welcome if that changes.

## Features

### Live Session Table
- Status indicators: Running, Done, Failed, **SPIN** (spinning detected)
- Project name, model, duration, iteration count
- Cost with color coding (green < $5, yellow < $15, red > $15)
- Context fill percentage
- Tool calls per minute (spin indicator)

### Cost Panel
- Per-session cost (live updating)
- Burn rate ($/minute)
- Today's total across all sessions
- Top 3 costliest sessions

### Context Health Panel
- Window fill % with visual bar
- Compaction count
- Token efficiency (output / total %)
- Cache hit rate

### Alerts Panel
- Spin detection (repeated tool calls, error echo, cost velocity)
- Budget threshold warnings (configurable %)
- Stall detection (no file edits despite ongoing activity, shown as a warning)
- Loop-ended alerts when an [LTF trace](#ltf-trace-integration) reports `spin_detected`, `stall_detected`, `error`, or `budget_exhausted`

### Session Detail View
Press `Enter` on a session for a full-screen breakdown: complete token
counts (with thousands separators), full file-changed list, full error
history, spin/warning reasons, and timing — everything the live table
only summarizes.

### Filter
Press `/` to narrow the live session table by project name
(case-insensitive substring match). `Enter` commits the filter, `Esc`
clears it. Filtering only applies to the live table — see History for
completed sessions.

### Session History
Press `h` for a dedicated view of **completed** sessions, separate from
the live table (which now shows active sessions only). Each row is
classified as Done, Failed, Killed, or Warned, plus a cost-over-time
sparkline and aggregate totals for the session.

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` | Quit |
| `?` | Toggle help |
| `Tab` | Cycle panel focus |
| `↑/k` `↓/j` | Navigate sessions |
| `Enter` | Open session detail |
| `/` | Filter live sessions by project name |
| `h` | Toggle session history view |
| `K` | Kill the selected/open session (SIGTERM) |
| `e` | Export the selected/open session as JSON |
| `Esc` | Close overlay / clear filter |

## Configuration

Default config path: `~/.config/loopctl/config.yaml`

```bash
loopctl config          # show the config path and whether it exists
loopctl config init     # create a default config file (fails if one exists)
loopctl config validate # check the config for issues without launching the TUI
```

```yaml
refresh_rate: "1s"      # how often the dashboard redraws (Go duration syntax)

budget:
  per_session_usd: 20.0  # sessions above this cost show a budget alert
  per_day_usd: 200.0     # daily cap across all sessions (local midnight to local midnight)
  warn_at_percent: 80    # warn once cost crosses this % of per_session_usd

spin:
  repeated_calls: 3           # same tool+input called N times in a row -> SPIN
  error_echo: 3                # same error message N times in a row -> SPIN
  stall_minutes: 10            # no file edits for N minutes despite activity -> warning
  cost_velocity_per_min: 2.0   # $/min burn rate that triggers a warning

sources:
  claude_code: "auto"    # auto | disabled
  codex: "auto"          # auto | disabled

logging:
  level: "info"          # debug | info | warn | error
```

### Environment Variable Overrides

| Variable | Description |
|----------|-------------|
| `LOOPCTL_BUDGET_PER_SESSION` | Per-session budget threshold ($) |
| `LOOPCTL_BUDGET_PER_DAY` | Daily budget threshold ($) |
| `LOOPCTL_LOG_LEVEL` | Log level (debug/info/warn/error) |

A malformed or invalid config never blocks LoopCtl from launching — problems
are reported to stderr and the affected values fall back to their defaults.
Run `loopctl config validate` to see exactly what would be corrected.

## LTF Trace Integration

If a project has the [LTF](https://github.com/loop-eng/ltf) Claude Code
adapter installed (writing `.loop/trace.ltf.jsonl`), LoopCtl automatically
enriches that session's display with LTF-verified data: a true
act→verify→decide iteration count (instead of a raw tool-call count) and
the loop's termination reason (`goal_met`, `budget_exhausted`,
`spin_detected`, etc.), shown as a reason-aware status icon. LTF is purely
additive — cost, tokens, and files-changed always come from the native
session log, never from the trace. Currently Claude Code only; Codex has no
LTF adapter yet.

## Spin Detection

LoopCtl inherits four battle-tested heuristics from [LoopGuard](https://github.com/loop-eng/loopguard):

| Heuristic | What It Detects | Default Threshold |
|-----------|----------------|-------------------|
| Repeated Tool Calls | Same tool+input called N times | 3 |
| Error Echo | Same error message repeated N times | 3 |
| Stall Detection | No file edits despite ongoing activity | 10 minutes |
| Cost Velocity | Burn rate exceeding $/min threshold | $2.00/min |

## Supported Models & Pricing

| Model | Input $/MTok | Output $/MTok |
|-------|-------------|---------------|
| claude-opus-4-{6,7,8} | $5.00 | $25.00 |
| claude-sonnet-4-{5,6} | $3.00 | $15.00 |
| claude-haiku-4-5 | $1.00 | $5.00 |
| gpt-5.5 | $5.00 | $30.00 |
| gpt-4.1 | $2.00 | $8.00 |
| gpt-4.1-mini | $0.40 | $1.60 |
| o4-mini | $1.10 | $4.40 |
| o3 | $2.00 | $8.00 |
| gemini-2.5-pro | $1.25 | $10.00 |
| gemini-2.5-flash | $0.15 | $0.60 |

Unknown models fall back to Sonnet-tier pricing ($3/$15).

## Architecture

```
Discovery (claude/codex) → Tailer (poll) → Parser (JSONL + LTF) → Metrics (cost/spin/context) → TUI (bubbletea v2)
```

- **Discovery**: Scans `~/.claude/projects/` and `~/.codex/sessions/` every 30s. Detects active sessions via `pgrep`/`lsof`.
- **Tailing**: Offset-tracked incremental file reads every 2s (polling, not filesystem-event-based).
- **Parsers**: Normalized event extraction from Claude Code and Codex JSONL formats. Two-generation request dedup prevents double-counting tokens. A separate LTF parser enriches sessions when a `.loop/trace.ltf.jsonl` adapter trace is present.
- **Cost Calculator**: Longest-prefix model matching with embedded pricing table.
- **Spin Detector**: Per-session stateful detector with circular buffers (window size 50).
- **Context Tracker**: Estimates fill % from token usage, detects compactions from input drops.
- **Session Store**: Thread-safe aggregate with snapshot-based data flow to the TUI.
- **TUI**: bubbletea v2 with lipgloss v2 styling. Configurable tick refresh via `tea.Tick` (`refresh_rate` in config, default 1s).

## Troubleshooting

**No sessions showing up.** Confirm `~/.claude/projects/` (or
`~/.codex/sessions/`) exists and has `.jsonl` files modified in the last
24 hours — LoopCtl only surfaces recently-active sessions. Check
`sources.claude_code`/`sources.codex` aren't set to `disabled` in your
config (`loopctl config validate` will flag this).

**Cost looks wrong for a model.** Unknown model strings fall back to
Sonnet-tier pricing ($3/$15 per MTok) — see the
[pricing table](#supported-models--pricing) for what's recognized. A model
released after your LoopCtl version won't have an exact entry yet.

**Quitting.** Press `q` or `Ctrl+C` in the terminal running LoopCtl to
exit normally. Sending a signal from outside that terminal (`kill`, a
process manager, an unattended script) also triggers a clean shutdown —
LoopCtl exits the dashboard and stops background polling either way.

**Demo data left behind.** If a `demo/trial.sh` or manual `loopctl-demo`
run didn't clean up after itself, run `bash demo/cleanup.sh`.

## Part of the loop-eng Ecosystem

| Project | What It Does | Status |
|---------|-------------|--------|
| [LTF](https://github.com/loop-eng/ltf) | Loop Trace Format — open standard for agent loop telemetry | Shipped |
| [LoopGuard](https://github.com/loop-eng/loopguard) | Circuit breaker daemon — stop runaways before they burn your budget | Shipped |
| **LoopCtl** | **htop for AI agents — live terminal dashboard** | **Shipped** |
| [Kit](https://github.com/loop-eng/kit) | Scaffold production-ready agent loops in 30s | In development |
| [Loop-Bench](https://github.com/loop-eng/loop-bench) | SWE-bench for loop designs | In development |
| [LoopRelay](https://github.com/loop-eng/looprelay) | Wireshark for agent loops — record, replay, debug | In development |

## Development

```bash
make build     # Build binary
make test      # Run tests with race detector
make lint      # Run golangci-lint
make run       # Build and run
make install   # Install to $GOPATH/bin
```

## License

MIT — see [LICENSE](LICENSE).
