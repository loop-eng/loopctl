package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RefreshRate string        `yaml:"refresh_rate"`
	Budget      BudgetConfig  `yaml:"budget"`
	Spin        SpinConfig    `yaml:"spin"`
	Sources     SourcesConfig `yaml:"sources"`
	Logging     LoggingConfig `yaml:"logging"`
}

type BudgetConfig struct {
	PerSessionUSD float64 `yaml:"per_session_usd"`
	PerDayUSD     float64 `yaml:"per_day_usd"`
	WarnAtPercent int     `yaml:"warn_at_percent"`
}

type SpinConfig struct {
	RepeatedCalls      int     `yaml:"repeated_calls"`
	ErrorEcho          int     `yaml:"error_echo"`
	StallMinutes       int     `yaml:"stall_minutes"`
	CostVelocityPerMin float64 `yaml:"cost_velocity_per_min"`
}

type SourcesConfig struct {
	ClaudeCode string   `yaml:"claude_code"`
	Codex      string   `yaml:"codex"`
	Custom     []string `yaml:"custom"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

// ValidationIssue describes a single problem found in a loaded config,
// after defaults and env overrides have already been applied.
type ValidationIssue struct {
	Field    string
	Message  string
	Severity string // "warning" | "error"
}

const minRefreshRate = 100 * time.Millisecond

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "loopctl")
}

// Load reads the config at path (or the default path if empty), applies
// defaults and env overrides, and validates the result.
//
// Load always returns a usable *Config — a malformed YAML file or an
// invalid field value never blocks the caller from launching the TUI.
// Those problems are reported via the returned []ValidationIssue instead.
// The one exception is the oversized-file guard, which is a resource-safety
// limit rather than a content problem and remains a hard error, along with
// genuine I/O errors reading an existing file (e.g. permission denied).
func Load(path string) (*Config, []ValidationIssue, error) {
	cfg := Default()

	if path == "" {
		path = filepath.Join(configDir(), "config.yaml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnvOverrides(cfg)
			issues := Validate(cfg)
			return cfg, issues, nil
		}
		return nil, nil, fmt.Errorf("reading config: %w", err)
	}

	if len(data) > 1<<20 {
		return nil, nil, fmt.Errorf("config file too large: %d bytes", len(data))
	}

	var issues []ValidationIssue

	parsed := Default()
	if err := yaml.Unmarshal(data, parsed); err != nil {
		issues = append(issues, ValidationIssue{
			Field:    "(file)",
			Message:  fmt.Sprintf("could not parse YAML, using defaults: %v", err),
			Severity: "error",
		})
		parsed = Default()
	}
	cfg = parsed

	applyDefaults(cfg)
	applyEnvOverrides(cfg)
	issues = append(issues, Validate(cfg)...)
	return cfg, issues, nil
}

// Validate checks cfg for out-of-range or unrecognized values, correcting
// them in place to a safe fallback and returning one ValidationIssue per
// correction. Call after applyDefaults/applyEnvOverrides.
func Validate(cfg *Config) []ValidationIssue {
	var issues []ValidationIssue
	defaults := Default()

	if s := normalizeSourceMode(cfg.Sources.ClaudeCode); s == "" {
		issues = append(issues, ValidationIssue{
			Field:    "sources.claude_code",
			Message:  fmt.Sprintf("invalid value %q, must be \"auto\" or \"disabled\" — using \"auto\"", cfg.Sources.ClaudeCode),
			Severity: "warning",
		})
		cfg.Sources.ClaudeCode = "auto"
	} else {
		cfg.Sources.ClaudeCode = s
	}

	if s := normalizeSourceMode(cfg.Sources.Codex); s == "" {
		issues = append(issues, ValidationIssue{
			Field:    "sources.codex",
			Message:  fmt.Sprintf("invalid value %q, must be \"auto\" or \"disabled\" — using \"auto\"", cfg.Sources.Codex),
			Severity: "warning",
		})
		cfg.Sources.Codex = "auto"
	} else {
		cfg.Sources.Codex = s
	}

	if cfg.Budget.WarnAtPercent < 1 || cfg.Budget.WarnAtPercent > 100 {
		issues = append(issues, ValidationIssue{
			Field:    "budget.warn_at_percent",
			Message:  fmt.Sprintf("%d is out of range (1-100) — using %d", cfg.Budget.WarnAtPercent, defaults.Budget.WarnAtPercent),
			Severity: "warning",
		})
		cfg.Budget.WarnAtPercent = defaults.Budget.WarnAtPercent
	}

	if cfg.Budget.PerSessionUSD < 0 {
		issues = append(issues, ValidationIssue{
			Field:    "budget.per_session_usd",
			Message:  fmt.Sprintf("%.2f is negative — using 0 (no cap)", cfg.Budget.PerSessionUSD),
			Severity: "warning",
		})
		cfg.Budget.PerSessionUSD = 0
	}

	if cfg.Budget.PerDayUSD < 0 {
		issues = append(issues, ValidationIssue{
			Field:    "budget.per_day_usd",
			Message:  fmt.Sprintf("%.2f is negative — using 0 (no cap)", cfg.Budget.PerDayUSD),
			Severity: "warning",
		})
		cfg.Budget.PerDayUSD = 0
	}

	if cfg.Spin.StallMinutes < 0 {
		issues = append(issues, ValidationIssue{
			Field:    "spin.stall_minutes",
			Message:  fmt.Sprintf("%d is negative — using %d", cfg.Spin.StallMinutes, defaults.Spin.StallMinutes),
			Severity: "warning",
		})
		cfg.Spin.StallMinutes = defaults.Spin.StallMinutes
	}

	if cfg.Spin.CostVelocityPerMin < 0 {
		issues = append(issues, ValidationIssue{
			Field:    "spin.cost_velocity_per_min",
			Message:  fmt.Sprintf("%.2f is negative — using %.2f", cfg.Spin.CostVelocityPerMin, defaults.Spin.CostVelocityPerMin),
			Severity: "warning",
		})
		cfg.Spin.CostVelocityPerMin = defaults.Spin.CostVelocityPerMin
	}

	if cfg.Spin.RepeatedCalls < 1 {
		issues = append(issues, ValidationIssue{
			Field:    "spin.repeated_calls",
			Message:  fmt.Sprintf("%d must be >= 1 — using %d", cfg.Spin.RepeatedCalls, defaults.Spin.RepeatedCalls),
			Severity: "warning",
		})
		cfg.Spin.RepeatedCalls = defaults.Spin.RepeatedCalls
	}

	if cfg.Spin.ErrorEcho < 1 {
		issues = append(issues, ValidationIssue{
			Field:    "spin.error_echo",
			Message:  fmt.Sprintf("%d must be >= 1 — using %d", cfg.Spin.ErrorEcho, defaults.Spin.ErrorEcho),
			Severity: "warning",
		})
		cfg.Spin.ErrorEcho = defaults.Spin.ErrorEcho
	}

	switch strings.ToLower(strings.TrimSpace(cfg.Logging.Level)) {
	case "debug", "info", "warn", "error":
		cfg.Logging.Level = strings.ToLower(strings.TrimSpace(cfg.Logging.Level))
	default:
		issues = append(issues, ValidationIssue{
			Field:    "logging.level",
			Message:  fmt.Sprintf("invalid value %q, must be one of debug|info|warn|error — using %q", cfg.Logging.Level, defaults.Logging.Level),
			Severity: "warning",
		})
		cfg.Logging.Level = defaults.Logging.Level
	}

	if d, err := time.ParseDuration(cfg.RefreshRate); err != nil {
		issues = append(issues, ValidationIssue{
			Field:    "refresh_rate",
			Message:  fmt.Sprintf("invalid duration %q — using %q", cfg.RefreshRate, defaults.RefreshRate),
			Severity: "warning",
		})
		cfg.RefreshRate = defaults.RefreshRate
	} else if d < minRefreshRate {
		issues = append(issues, ValidationIssue{
			Field:    "refresh_rate",
			Message:  fmt.Sprintf("%q is below the minimum (%s) — using %s", cfg.RefreshRate, minRefreshRate, minRefreshRate),
			Severity: "warning",
		})
		cfg.RefreshRate = minRefreshRate.String()
	}

	return issues
}

// normalizeSourceMode returns "auto" or "disabled" (case/whitespace
// normalized), or "" if v is neither.
func normalizeSourceMode(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "auto":
		return "auto"
	case "disabled":
		return "disabled"
	default:
		return ""
	}
}

func applyDefaults(cfg *Config) {
	defaults := Default()
	if cfg.RefreshRate == "" {
		cfg.RefreshRate = defaults.RefreshRate
	}
	if cfg.Budget.WarnAtPercent <= 0 {
		cfg.Budget.WarnAtPercent = defaults.Budget.WarnAtPercent
	}
	if cfg.Spin.RepeatedCalls <= 0 {
		cfg.Spin.RepeatedCalls = defaults.Spin.RepeatedCalls
	}
	if cfg.Spin.ErrorEcho <= 0 {
		cfg.Spin.ErrorEcho = defaults.Spin.ErrorEcho
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = defaults.Logging.Level
	}
}

func applyEnvOverrides(cfg *Config) {
	parseFloat := func(key string, target *float64) {
		v := os.Getenv(key)
		if v == "" {
			return
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			slog.Warn("invalid env var value, ignoring", "key", key, "value", v, "error", err)
			return
		}
		*target = f
	}
	parseFloat("LOOPCTL_BUDGET_PER_SESSION", &cfg.Budget.PerSessionUSD)
	parseFloat("LOOPCTL_BUDGET_PER_DAY", &cfg.Budget.PerDayUSD)
	if v := os.Getenv("LOOPCTL_LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
}

func DefaultPath() string {
	return filepath.Join(configDir(), "config.yaml")
}

func Default() *Config {
	return &Config{
		RefreshRate: "1s",
		Budget: BudgetConfig{
			PerSessionUSD: 20.0,
			PerDayUSD:     200.0,
			WarnAtPercent: 80,
		},
		Spin: SpinConfig{
			RepeatedCalls:      3,
			ErrorEcho:          3,
			StallMinutes:       10,
			CostVelocityPerMin: 2.0,
		},
		Sources: SourcesConfig{
			ClaudeCode: "auto",
			Codex:      "auto",
			Custom:     []string{},
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
}
