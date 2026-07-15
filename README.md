# LoopCtl

[![CI](https://github.com/loop-eng/loopctl/actions/workflows/ci.yaml/badge.svg)](https://github.com/loop-eng/loopctl/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/loop-eng/loopctl)](https://goreportcard.com/report/github.com/loop-eng/loopctl)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**htop for AI coding agents** — a live terminal dashboard that monitors all your Claude Code, Codex, and Gemini CLI sessions with real-time cost tracking, context health, and spin detection.

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
│ [q]uit [K]ill [e]xport [?]help [tab]panel [enter]detail                │
└──────────────────────────────────────────────────────────────────────────┘
```

## The Problem

You're running 3 Claude Code sessions, a Codex task, and maybe Gemini CLI — simultaneously. Right now you have no way to see:

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
- Stall detection (no file edits despite ongoing activity)

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `q` / `Ctrl+C` | Quit |
| `?` | Toggle help |
| `Tab` | Cycle panel focus |
| `↑/k` `↓/j` | Navigate sessions |
| `Enter` | Session detail |
| `Esc` | Close overlay |

## Configuration

Default config path: `~/.config/loopctl/config.yaml`

```yaml
refresh_rate: "1s"

budget:
  per_session_usd: 20.0
  per_day_usd: 200.0
  warn_at_percent: 80

spin:
  repeated_calls: 3
  error_echo: 3
  stall_minutes: 10
  cost_velocity_per_min: 2.0

sources:
  claude_code: "auto"
  codex: "auto"

logging:
  level: "info"
```

### Environment Variable Overrides

| Variable | Description |
|----------|-------------|
| `LOOPCTL_BUDGET_PER_SESSION` | Per-session budget threshold ($) |
| `LOOPCTL_BUDGET_PER_DAY` | Daily budget threshold ($) |
| `LOOPCTL_LOG_LEVEL` | Log level (debug/info/warn/error) |

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
Discovery (claude/codex) → Watcher (fsnotify) → Parser (JSONL) → Metrics (cost/spin/context) → TUI (bubbletea v2)
```

- **Discovery**: Scans `~/.claude/projects/` and `~/.codex/sessions/` every 30s. Detects active sessions via `pgrep`/`lsof`.
- **Parsers**: Normalized event extraction from Claude Code and Codex JSONL formats. Two-generation request dedup prevents double-counting tokens.
- **Cost Calculator**: Longest-prefix model matching with embedded pricing table.
- **Spin Detector**: Per-session stateful detector with circular buffers (window size 50).
- **Context Tracker**: Estimates fill % from token usage, detects compactions from input drops.
- **Session Store**: Thread-safe aggregate with snapshot-based data flow to the TUI.
- **TUI**: bubbletea v2 with lipgloss v2 styling. 1-second tick refresh via `tea.Tick`.

## Part of the loop-eng Ecosystem

| Project | What It Does | Status |
|---------|-------------|--------|
| [LTF](https://github.com/loop-eng/ltf) | Loop Trace Format — open standard for agent loop telemetry | Shipped |
| [LoopGuard](https://github.com/loop-eng/loopguard) | Circuit breaker daemon — stop runaways before they burn your budget | Shipped |
| **LoopCtl** | **htop for AI agents — live terminal dashboard** | **Shipped** |
| Kit | Scaffold production-ready agent loops in 30s | Planned |
| Loop-Bench | SWE-bench for loop designs | Planned |
| LoopReplay | Wireshark for agent loops — record, replay, debug | Planned |

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
