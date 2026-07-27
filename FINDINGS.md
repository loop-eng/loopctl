# LoopCtl Bug Audit Findings

Generated: 2026-07-28
Method: Retroactive backfill of 3 informal passes (code review, critical
sweep, security sweep — commits `37f6a06` through `dd90e4e`) plus Phase 2
work: Pass 3 concurrency stress testing, Pass 6 adversarial testing, Pass 4
deferred-item cleanup, Pass 12 fuzz testing, and Pass 13 CI hardening, all
against LoopCtl's 59 Go files / ~8,600 lines.

## Summary table

| Severity | Count | Confirmed | Fixed |
|----------|-------|-----------|-------|
| Critical | 3     | 3         | 3     |
| High     | 15    | 15        | 15    |
| Medium   | 20    | 20        | 19    |
| Low      | 12    | 12        | 12    |
| **Total**| 50    | 50        | 49    |

The one non-"Fixed" item is P4 (Snapshot's three separately-locked reads),
recorded as an accepted design tradeoff rather than a defect — see its
entry below for why closing it further isn't worth the cost.

## E2E Verification

- `bash demo/test_e2e.sh` — **26/26 PASS**
- `go build ./...` — clean
- `go vet ./...` — clean
- `golangci-lint run ./...` — **0 issues**
- `go test -race -count=1 ./...` — all 9 packages **PASS**, 159 test
  functions (up from 126 before Phase 2)
- `go test -fuzz=FuzzClaudeParser -fuzztime=30s ./internal/parser/` —
  0 crashes across ~650K+ executions
- `go test -fuzz=FuzzCodexParser -fuzztime=30s ./internal/parser/` —
  0 crashes across ~3.8M+ executions
- `go test -fuzz=FuzzTailerReadNewLines -fuzztime=30s ./internal/source/` —
  0 crashes (file-I/O bound, ~7K+ executions)
- `govulncheck ./...` — 0 vulnerabilities reachable from LoopCtl's code
  (2 unreachable stdlib CVEs noted below, not app-code findings)
- `gh issue list --state open` — 0 (was 6 at the start of Phase 2)

---

## Pass 1-2/Review + Critical Sweep (commit `37f6a06`)

18 items found across a review pass and a critical-sweep pass, fixed in one
commit alongside the initial demo simulator and E2E suite.

### R1: Panic on narrow terminal (Critical)
- **File:** `internal/panel/alerts.go`
- **Status:** FIXED
- **Description:** A negative slice index was reachable when the alerts
  panel's computed width went below the space needed for its content.
- **Impact:** LoopCtl crashed outright on terminals below a certain width.
- **Fix:** Added a floor/clamp before slicing.

### R2: CacheHitRate/TokenEfficiency accumulate incorrectly (Medium)
- **File:** `internal/metrics/context.go`
- **Status:** FIXED
- **Description:** The two metrics were overwritten per-event instead of
  accumulated across the session.
- **Impact:** Displayed cache-hit-rate and token-efficiency numbers were
  wrong for any multi-event session (the vast majority of real sessions).
- **Fix:** Changed to running accumulation.

### R3: `--verbose` logging silently discarded (Medium)
- **File:** `internal/cli/root.go`
- **Status:** FIXED
- **Description:** The logger was wired to `io.Discard` regardless of the
  `--verbose` flag.
- **Impact:** `--verbose` had zero effect — a debugging feature that never
  worked from day one.
- **Fix:** Route to `os.Stderr` when verbose is set. Now covered by
  `TestSetupLoggerVerboseWritesToStderr` (Phase 2).

### R4: Stall detection dead code (Medium)
- **File:** `internal/metrics/spin.go`
- **Status:** FIXED
- **Description:** `checkStall`'s result was computed but never assigned
  into the returned `SpinResult`.
- **Impact:** Stalled sessions never surfaced a warning.
- **Fix:** Wired the result into `IsSpinning`/`HasWarnings` (later refined
  further by F10 below).

### R5: TopSessions sorted by activity, not cost (Low)
- **File:** `internal/metrics/session.go`
- **Status:** FIXED
- **Description:** The "top sessions" cost panel used the wrong sort key.
- **Impact:** Cosmetic — wrong ordering in a UI list, no data loss.
- **Fix:** Sort by `Cost` descending.

### R6: Kill/Export keybindings unimplemented (High)
- **File:** `internal/app/model.go`
- **Status:** FIXED
- **Description:** `K` and `e` were documented in the help bar but did
  nothing.
- **Impact:** Two of the tool's advertised core actions were non-functional.
- **Fix:** Implemented `handleKill`/`handleExport` (further hardened in
  S1-S4 and, in Phase 2, given user feedback — see P2).

### R7: go.mod Go version mismatched CI matrix (Low)
- **Status:** FIXED — infra/tooling only.

### R8: Compaction false positive on model switch (Medium)
- **File:** `internal/metrics/context.go`
- **Status:** FIXED
- **Description:** Switching models mid-session reset `prevContextLoad` to
  0, which the compaction-detection logic misread as a real compaction
  event.
- **Impact:** A cosmetic model switch (e.g. user manually changes CLI tool)
  showed up as a spurious compaction in the context panel.
- **Fix:** Compaction detection now ignores the reset caused by a model
  switch. Re-verified end-to-end in Phase 2's
  `TestIntegrationModelSwitchMidStream`.

### R9: Double table allocation in NewSessionPanel (Low)
- **Status:** FIXED — performance/hygiene only, no observable bug.

### R10: Dead type aliases in messages.go (Low)
- **Status:** FIXED at the time — note that `internal/app/messages.go`
  regressed to an empty (but still present) file afterward; fully deleted
  in Phase 2 (see P15).

### C1: Missing bounds checks on panel width/height (High)
- **Files:** `internal/panel/cost.go`, `context.go`, `alerts.go`
- **Status:** FIXED
- **Description:** Several panels assumed a minimum width/height when
  slicing/padding content.
- **Impact:** Crashes on aggressive terminal resize.
- **Fix:** Added floor clamps throughout.

### C2: `SpinConfig.WindowSize=0` divide-by-zero (Critical)
- **File:** `internal/metrics/spin.go`
- **Status:** FIXED
- **Description:** A zero window size (reachable via a misconfigured or
  future config path) divided by zero in a percentage calculation.
- **Impact:** Guaranteed panic.
- **Fix:** `NewSpinDetector` now defaults `WindowSize` to 50 when `<= 0`.

### C3: `ContextBar` negative percentage guard (Medium)
- **Status:** FIXED — guarded against a negative fill percent producing an
  invalid bar width.

### C4: Collector discarded lines on partial tailer error (High)
- **File:** `internal/source/collector.go`
- **Status:** FIXED
- **Description:** A tail-read error caused the whole batch of lines from
  that cycle to be dropped instead of just skipping the failed read.
- **Impact:** Real event data loss under transient read errors.
- **Fix:** Errors are now logged and the loop continues; re-verified by
  Phase 2's `TestAdversarialBinaryGarbageMixedWithValidLines` and
  `TestIntegrationMalformedJSONLMixedWithValid`, which specifically prove a
  bad line doesn't take good lines around it down with it.

### C5: Stale tailers/parsers grow unbounded (High)
- **File:** `internal/source/collector.go`
- **Status:** FIXED
- **Description:** `runDiscovery` never pruned tailer/parser map entries
  for sessions that disappeared.
- **Impact:** Slow memory leak for any long-running loopctl process
  monitoring a machine with session churn.
- **Fix:** Added pruning against the freshly-discovered ID set every cycle.
  Covered by `TestRunDiscoveryPrunesStaleLTFTailers` and, for the
  session-disappearing case specifically, Phase 2's
  `TestAdversarialSessionFileDeletedWhileTailing`.

### C6: Alert panel width clamping (Medium) / C7: handleExport error handling (Medium) / C8: missing updatePanels on resize (Medium)
- **Status:** FIXED — see commit `37f6a06` body for detail; superseded by
  the Phase 1 rewrite of `internal/app/model.go` and further hardened for
  user feedback in Phase 2 (P2).

---

## Critical Sweep, 7 bugs (commit `ffe2827`)

### F1: Data race on `Collector.cancel` (High)
- **File:** `internal/source/collector.go`
- **Status:** FIXED
- **Description:** `Start`/`Close` read and wrote `c.cancel` without
  synchronization.
- **Impact:** A genuine data race, catchable by `-race`, between
  starting/stopping the collector.
- **Fix:** Added `cancelMu`. Phase 2 went further and added a `sync.WaitGroup`
  so `Close` blocks until the background loop has fully exited — see P7.

### F2: `TopSessions` panics on negative `n` (Critical)
- **Status:** FIXED — guarded with an early return for `n <= 0`.

### F3: Empty model string uses no pricing fallback (Medium)
- **Status:** FIXED — defaults to Sonnet-tier pricing when the model
  string is empty.

### F4: Codex parser double-counts on the wrong dedup shape (High)
- **File:** `internal/parser/codex.go`
- **Status:** FIXED
- **Description:** Codex parsing lacked the two-generation dedup set the
  Claude parser already had, allowing re-processed/duplicate log lines to
  double-count tokens.
- **Impact:** Inflated cost/token figures for Codex sessions.
- **Fix:** Added the same `currentGen`/`previousGen` rotation as
  `ClaudeParser`. Now fuzz-tested (`FuzzCodexParser` asserts
  `currentCount` never exceeds `maxSeenRequests`).

### F5: `parseTimestamp` zero-time cascade (High)
- **File:** `internal/parser/claude.go`
- **Status:** FIXED
- **Description:** A malformed timestamp parsed to the zero `time.Time`,
  which downstream stall-detection logic read as "an eternity since last
  activity."
- **Impact:** False stall alerts triggered by a single malformed log line.
- **Fix:** Falls back to `time.Now()` on parse failure. Fuzz-tested: every
  event returned by `FuzzClaudeParser` is asserted to have a non-zero
  timestamp.

### F6: Token attribution shifts to the wrong event (High)
- **File:** `internal/parser/claude.go`
- **Status:** FIXED
- **Description:** If the first content block in a multi-block assistant
  message failed to unmarshal, the usage tokens meant for it silently
  attached to a later block instead.
- **Impact:** Misattributed cost/token data — the number is right in
  aggregate but assigned to the wrong tool call, corrupting any
  per-tool-call analysis.
- **Fix:** Added a `tokensAssigned` guard so tokens attach exactly once, to
  the first successfully-parsed block.

### F7: `ExportDoneMsg` unhandled in `Update` (Medium)
- **Status:** FIXED at the time (added a case, but with no user-visible
  effect); Phase 2 closed the loop fully — see P2.

---

## Context Fill Fix (commit `9895f3f`)

### F8: Context fill always shows 0% (High)
- **File:** `internal/metrics/context.go`
- **Status:** FIXED
- **Description:** Fill percentage was computed from `input_tokens` alone
  (typically ~3 for a cached request) instead of the full context load
  (`input + cache_read + cache_write`).
- **Impact:** The context-health panel — one of LoopCtl's headline
  features — showed 0% for essentially every real session.
- **Fix:** Sum all three token categories for the fill-percent denominator.
  Verified against real Claude Code session data at the time; re-verified
  in Phase 2's deterministic integration suite.

---

## Spin Detection False-Positive Fix (commit `cd9e530`)

### F9: Repeated-tool-call heuristic ignores time window (High)
- **File:** `internal/metrics/spin.go`
- **Status:** FIXED
- **Description:** The heuristic counted matching tool-call fingerprints
  anywhere in the 50-entry circular buffer, regardless of how far apart in
  time they occurred.
- **Impact:** Running the same command 3 times over a 3-hour productive
  session tripped a false SPIN flag. Verified against 36 real local Claude
  Code sessions: **12/36 were falsely flagged before the fix, 1/36 after**
  (the one remaining flag was the intentional demo spin scenario). This is
  arguably the single most user-visible bug in the project's history —
  rated High rather than Medium specifically because it actively broke the
  product's core value proposition for real users, even though it never
  crashed or lost data.
- **Fix:** Added a 5-minute window to `checkRepeatedTools`.

### F10: Stall downgraded from critical to warning (Medium)
- **Status:** FIXED — stall now sets `HasWarnings` (severity "warning"),
  not `IsSpinning` (severity "critical"), matching that a stall is a normal
  research phase, not necessarily a problem.

---

## Security Hardening + Data Integrity (commit `dd90e4e`)

### S1: Export files world-readable (High)
- **File:** `internal/app/model.go`
- **Status:** FIXED — `0644`→`0600` for files, `0755`→`0700` for the
  exports directory. Session exports can contain tool inputs/outputs,
  file paths, and error messages — not something to leave world-readable.

### S2: Export path lacks defense-in-depth sanitization (High)
- **Status:** FIXED — `filepath.Base(sessionID)` applied before joining
  into the export path.

### S3: Kill handler missing PID validation (High)
- **File:** `internal/app/model.go`
- **Status:** FIXED — rejects PID ≤ 1 and verifies the process still
  exists (`Signal(0)`) before sending `SIGTERM`. Phase 2 (P2) built on this
  by surfacing the outcome to the user instead of failing silently.

### S4: Symlink TOCTOU in file reads (High)
- **Files:** `internal/source/claude.go`, `internal/source/tailer.go`
- **Status:** FIXED — `os.Lstat` + `ModeSymlink` check before opening any
  discovered session file, preventing a maliciously or accidentally
  symlinked "session file" from causing LoopCtl to read an arbitrary path.
  Dedicated regression test added in Phase 2:
  `TestTailerSkipsSymlinks`.

### S5: `FilesChanged` map unbounded (Medium)
- **Status:** FIXED — capped at 10,000 entries per session to prevent
  memory exhaustion from a pathological session touching an enormous
  number of distinct files.

### D1: `CacheHitRate` wrong denominator (Medium)
- **Status:** FIXED — `cacheRead/(input+cacheRead)` instead of
  `cacheRead/input`, which had been producing inflated (sometimes >100%)
  values.

### D2: Tailer doesn't advance offset on non-EOF errors (High)
- **File:** `internal/source/tailer.go`
- **Status:** FIXED — offset is now updated even on a non-EOF read error,
  preventing the same lines from being re-read and double-counted on the
  next cycle.

### D3: Parse errors completely silent (Low)
- **Status:** FIXED — logged at Debug level instead of discarded.

---

## Pass 3: Concurrency Audit (Phase 2)

Three latent issues were filed as GitHub issues (#1, #2, #3) after the
security-hardening pass above but before formal stress testing. This pass
closes all three with either a fix or a documented "safe by design"
conclusion — never left open once investigated, per the audit protocol's
principle that a disproven issue left open is its own kind of false
confidence.

### P4: `Collector.Snapshot()` combines three separately-locked reads (Low) — GitHub #1
- **File:** `internal/source/collector.go`
- **Status:** CONFIRMED — PARTIALLY FIXED / accepted design tradeoff
- **Description:** `Snapshot()` builds `Sessions` from one
  `SessionStore.Snapshot()` call, `Alerts` from a separate
  `Collector.mu`-guarded read, and (previously) `DailyTotal` from a third,
  independent `SessionStore.DailyTotal()` call. This is not a data race —
  each critical section is correctly synchronized on its own — but a write
  landing between sections could theoretically produce a `DataMsg` whose
  three fields never coexisted as one consistent state.
- **Impact:** In practice, bounded to a one-tick (1s default refresh)
  staleness window on values that are already display-only. Not observable
  as incorrect by a user watching the dashboard.
- **Fix:** `DailyTotal` is now computed by the new
  `metrics.DailyTotalFromSnapshot(snap)` from the *same* `snap` used to
  build `Sessions`, eliminating that specific inconsistency (verified by
  `TestDailyTotalFromSnapshotMatchesConcurrentReads` under concurrent
  writers). `Alerts` remains from a separately-timed read by design — it's
  computed on its own 2-second tick cadence inside `buildAlerts`, distinct
  from the TUI's render cadence, and merging them would mean recomputing
  alerts on every render tick for no real benefit. Accepted as the
  intentional tradeoff for a 1Hz-refreshed dashboard rather than pursued
  further.

### P5: Lock ordering — `buildAlerts` holds `Collector.mu` then acquires `SessionStore.mu` (Medium) — GitHub #2
- **Status:** SAFE BY DESIGN — no live deadlock today, confirmed by
  grepping every lock/unlock site in both `Collector` and `SessionStore`:
  the only place both locks are held simultaneously is `buildAlerts`,
  always in the order `Collector.mu` → `SessionStore.mu`. Nothing acquires
  them in the reverse order. Documented explicitly with comments on both
  `Collector.mu` and `SessionStore.mu` so a future contributor doesn't
  invert the order. Stress-tested under `-race` with concurrent
  `buildAlerts`/`ProcessEvent`/`Snapshot` calls for 2 seconds with a
  10-second deadlock timeout in
  `TestCollectorSnapshotAndBuildAlertsConcurrent` — completes cleanly.

### P6: `SpinResult.Reasons` shallow-copied in `Snapshot()` (Medium) — GitHub #3
- **File:** `internal/metrics/session.go`
- **Status:** CONFIRMED — FIXED
- **Description:** `SessionStore.Snapshot()`'s per-session copy (`cp := *s`)
  copied the `SpinResult` struct by value, but `Reasons []string`'s
  backing array was shared with the live session. Investigation confirmed
  `SpinDetector.Check()` always allocates a fresh `Reasons` slice per call
  and never mutates a previous call's backing array — so this was "safe by
  accident of the current implementation," not safe by contract. A future
  optimization reusing a pooled buffer (plausible, since `SpinDetector`
  already uses fixed-size ring buffers elsewhere) would silently
  reintroduce a real, `-race`-detectable bug.
- **Fix:** Added a defensive copy (`cp.Spin.Reasons =
  append([]string(nil), s.Spin.Reasons...)`), mirroring the existing
  `FilesChanged`/`ErrorMessages` deep-copy pattern in the same function.
  Turns "verify it stays fine" into a structural guarantee. Regression
  tested in `TestSessionStoreConcurrentStress`, which mutates a returned
  snapshot's `Reasons` slice from a background goroutine and asserts the
  mutation never leaks into a later `Snapshot()` call.

### P7: Collector restart-cycle race (Medium) — new finding, adversarial test A6
- **File:** `internal/source/collector.go`
- **Status:** CONFIRMED (by code reading) — FIXED
- **Description:** `Close()` called `cancel()` and returned immediately,
  without waiting for the background `loop()` goroutine to actually observe
  `ctx.Done()` and exit. A `Start()` called immediately afterward runs
  `runDiscovery()`/`processAllTails()` synchronously on the caller's
  goroutine — if the previous `loop()` goroutine hadn't exited yet and was
  itself mid-`runDiscovery()`/`processAllTails()` (only possible if `Close`
  happens to land while a 30s/2s ticker has just fired), both goroutines
  would mutate the unsynchronized `c.tailers`/`c.parsers` maps
  concurrently.
- **Impact:** Given the collector's actual ticker intervals (30s
  discovery, 2s tail), the window is narrow — the background goroutine is
  parked in a `select` for close to the entire interval between ticks — but
  "narrow" isn't "impossible," and a concurrent map write in Go can crash
  the whole process (`fatal error: concurrent map writes`), not merely
  trigger a `-race` warning. `TestCollectorRestartCycleRace` was run
  against the pre-fix code 5× and did not happen to reproduce the exact
  interleaving within its short deadlines, which is reported honestly here
  rather than overstated — the fix itself doesn't depend on having
  reproduced the race, since blocking `Close()` on the loop's exit is
  standard, unambiguously-correct practice for any type managing a
  background goroutine.
- **Fix:** Added a `sync.WaitGroup` — `Start()` calls `wg.Add(1)` before
  spawning `loop()`, `loop()` calls `defer wg.Done()`, and `Close()` now
  calls `wg.Wait()` after `cancel()`. `Close()` is safe to call from
  `defer` in `cli/root.go` as before (the loop's ctx-check-driven exit is
  effectively immediate, so `Wait()` adds negligible latency to shutdown).

---

## Pass 4: Deferred Items (Phase 2)

Three items were confirmed and filed but deliberately deferred during the
CVE/data-integrity pass, tagged for OSS contribution. All three are now
fixed rather than left open, per the audit protocol's "no indefinitely
deferred confirmed items" principle.

### P1: `DailyTotal` used UTC midnight, not local midnight (Medium) — GitHub #4
- **File:** `internal/metrics/session.go`
- **Status:** FIXED
- **Description:** `time.Now().Truncate(24 * time.Hour)` rounds down to
  the nearest 24-hour boundary from the Unix epoch in UTC — not local
  midnight, despite reading as "today" to anyone in a non-UTC timezone.
- **Impact:** For any user west of UTC, "today's total" flips over at a
  time that doesn't match their actual midnight (e.g. 4-8pm local time,
  depending on timezone), silently including or excluding sessions a user
  would reasonably expect on the other side of the line.
- **Fix:** Compute local midnight explicitly via
  `time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())`.
  Extracted into a shared `dailyCutoff()` helper used by both `DailyTotal`
  and the new `DailyTotalFromSnapshot` (P4) so they can never disagree on
  the day boundary.

### P2: `handleKill` gives no user feedback (Low) — GitHub #5
- **File:** `internal/app/model.go`
- **Status:** FIXED
- **Description:** Every path through `handleKill` — no session selected,
  session inactive, no PID, process already gone, signal failure, and
  success — returned `nil`, so the TUI never displayed anything different
  no matter what happened. The same was true of `ExportDoneMsg`, which was
  already being returned but discarded unread in `Update()`.
- **Impact:** Pressing `K` or `e` was a black box — a user watching the
  dashboard had no way to tell whether a kill signal was actually sent, or
  whether an export actually wrote a file, without checking externally
  (`ps`, the filesystem).
- **Fix:** Added a `model.KillDoneMsg{ProjectName, Err}` message type
  (mirroring the existing `ExportDoneMsg`), a `Model.statusMsg`/
  `statusExpiry` pair that renders in the footer for 4 seconds, and
  explicit early-feedback messages for the "no session selected" /
  "not active" / "no PID" cases that previously failed silently.

### P3: Session/history table columns overflow below ~84 columns (Medium) — GitHub #6
- **Files:** `internal/panel/sessions.go`, `internal/panel/history.go`
- **Status:** FIXED
- **Description:** Both tables used fixed per-column widths that summed to
  a fixed total (~84 and ~78 columns respectively) regardless of the
  actual terminal width, so any narrower terminal simply overflowed.
- **Impact:** On any terminal narrower than the fixed sum (a common laptop
  split-pane width), the dashboard rendered wider than the visible area.
- **Fix:** New `internal/panel/columns.go` with a `fitColumns` helper that
  shrinks lower-priority columns toward a documented per-column minimum
  before exceeding the terminal width, instead of ignoring it. Practical
  floor dropped from a hard ~84/~78 to ~61/~56 columns. Covered by
  `internal/panel/columns_test.go` across a range of widths including the
  regression case (83 columns, just under the old hard requirement).

---

## Pass 6: Adversarial Testing (Phase 2)

Built as `internal/source/adversarial_test.go`, 7 automated scenarios
(A1-A5, A10-A11) against the live pipeline — a gap noted at the start of
Phase 2, since prior "adversarial testing" (cited in `dd90e4e`'s commit
body) was manual and left no committed, repeatable test code.

| # | Scenario | Result |
|---|---|---|
| A1 | Binary garbage mixed with valid JSONL lines | PASS — parser errors on the garbage line only; surrounding valid lines still parse |
| A2 | Empty (0-byte) session file | PASS — baseline, no lines, no error |
| A3 | Single line far exceeding 1MB (mid-write, then terminated) | PASS — oversized content never reaches the parser; recovery after the next newline works and doesn't re-accumulate on repeated reads |
| A4 | Partial/truncated write at EOF (process killed mid-write) | PASS — buffered and correctly reassembled once the rest of the line arrives |
| A5 | 500 synthetic sessions, one tail cycle | PASS — completes in ~0.2-0.8s, well under the 2s tail-tick interval |
| A10 | Session file deleted while being tailed | PASS — `ReadNewLines` returns an error, not a panic; a fresh `Discover()` call no longer reports the deleted session |
| A11 | Session path replaced by a directory (and reversed) | PASS — no panic on either direction; `Lstat`'s symlink check doesn't misclassify a directory |

A6 (rapid start/stop) is covered separately in `collector_race_test.go` —
see P7. A7-A9 (config validation for negative/extreme values) were already
fixed as part of Phase 1's `config.Validate()` rewrite — confirmed via
`TestValidateNegativeBudget` and `TestValidateNegativeSpinFields`, not
re-implemented here. A8 (per-session `WindowSize` configurability) is
**N/A** — `SpinConfig.WindowSize` is intentionally not exposed via YAML;
confirmed by grep, not a gap.

A12 (two simultaneous `loopctl` instances) is a **documented design
decision**, not an automated test: LoopCtl has no daemon, no lock file, and
no shared mutable state between two OS processes — each has its own
independent in-memory `Collector`/`SessionStore`, and both only ever read
(never write) session log files. The one shared-mutable-state path is the
export file (`~/.config/loopctl/exports/<sessionID>.json`); `os.WriteFile`
opens with `O_TRUNC`, making a single-syscall write safe against a torn
file even if two instances export the same session at the same instant.
This is accepted as correct-by-design rather than built out as a
dual-process E2E harness, which would add real CI cost for a scenario with
no actual failure mode to catch.

---

## Pass 8/11/12/13: Test Coverage and CI Hardening (Phase 2)

### P9: `make fuzz` target was broken (Low)
- **File:** `Makefile`
- **Status:** FIXED
- **Description:** The pre-existing `fuzz` target used `-fuzz=Fuzz`, a
  prefix pattern. It silently did nothing before Phase 2 (zero `func Fuzz`
  functions existed in the repo despite the target's existence). Once
  `FuzzClaudeParser` and `FuzzCodexParser` were added to the same package
  in this phase, the same pattern started failing outright: `go test`
  refuses to fuzz when a pattern matches more than one `Fuzz` function.
- **Fix:** Target now runs each of the three fuzz functions individually,
  30s each.

### P11: No fuzz targets existed (Medium)
- **Status:** FIXED — `FuzzClaudeParser`, `FuzzCodexParser` (both in
  `internal/parser/fuzz_test.go`), `FuzzTailerReadNewLines`
  (`internal/source/fuzz_test.go`). 0 crashes across ~4.5M+ combined
  executions in local 15-30s runs; no crash corpus was produced (Go only
  persists failing inputs to `testdata/fuzz`, and none were found).

### P12: No deterministic, CI-independent integration tests (Medium)
- **Status:** FIXED — `internal/source/integration_test.go`, 8 scenarios
  driving the full Discoverer → Tailer → Parser → SessionStore → Collector
  pipeline against synthetic fixtures in `t.TempDir()`. Complements (does
  not replace) `live_test.go`, which is a valuable but CI-non-portable
  sanity check that skips cleanly when no real session data exists.
  Required two small, additive production constructors:
  `NewClaudeDiscovererAt`/`NewCodexDiscovererAt`, which take an explicit
  base directory instead of always resolving `$HOME` — also generally
  useful for a future `--claude-dir`-style flag.

### Pass 8: CLI edge cases (Low)
- **Status:** FIXED — `internal/cli/root_test.go` added: `--version`,
  `--help`, unknown flag, unknown subcommand, and full table-driven
  coverage of `setupLogger`'s level-mapping logic (previously untested —
  `--verbose` had already been broken once before, per R3 above, without
  any test that would have caught it).

### Pass 13: `govulncheck` not wired into CI (Low)
- **File:** `.github/workflows/ci.yaml`
- **Status:** FIXED — added a `vulncheck` job, gating the release job
  alongside `build`/`lint`. Local run before wiring it in: **0 reachable
  vulnerabilities** in LoopCtl's own code or call graph. Two vulnerabilities
  were reported in the Go 1.26.4 standard library itself
  (`GO-2026-4970`, `GO-2026-5856`, both fixed in stdlib 1.26.5) — neither
  is reachable from any code LoopCtl calls (confirmed via govulncheck's
  symbol-level analysis, not just package-level), so these are **N/A**
  findings, not app-code defects. Recommend bumping the CI Go version to
  1.26.5+ at the next convenient point as routine toolchain hygiene, not
  as a security fix for LoopCtl itself.

### P14: `readSessionMeta` swallows `bufio.Scanner` errors (Low)
- **File:** `internal/source/claude.go`
- **Status:** FIXED — logs at Debug level if `scanner.Err()` is non-nil
  after the scan loop (e.g. a line exceeding the 64KB metadata-scan
  buffer). Low severity — the function already falls back to safe defaults
  (`info.ModTime()`, `DecodeProjectDir`) — but previously left zero trail
  for diagnosing why a session's metadata came back empty.

### P15: Dead code (Low)
- **Status:** FIXED — removed `style.Header`, `style.Highlight` (both
  genuinely unreferenced; golangci-lint's `unused` linter doesn't flag
  unused *exported* identifiers by default, so these survived every prior
  lint pass undetected), the empty `internal/app/messages.go` file, and
  the write-only `Session.LastEvent` field (set by both discoverers,
  read by nothing).

### P16: No dedicated CLI tests before Phase 2 (Low)
- **Status:** FIXED — folded into Pass 8 above.

---

## Pass 7 & 9: N/A / Closed

- **Pass 7 (Live Enforcement):** N/A — LoopCtl is a read-only dashboard by
  design (per its own README positioning against LoopGuard, the
  enforcement daemon in the same ecosystem). Its only user-initiated
  process action is the manual `K` kill keybinding; there is no automated
  enforcement loop to test.
- **Pass 9 (Last Items):** Closed — no further items surfaced once Passes
  1-8, 11-13 above were completed; `gh issue list --state open` returns 0.
