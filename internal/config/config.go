package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

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

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "loopctl")
}

func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		path = filepath.Join(configDir(), "config.yaml")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnvOverrides(cfg)
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if len(data) > 1<<20 {
		return nil, fmt.Errorf("config file too large: %d bytes", len(data))
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	applyDefaults(cfg)
	applyEnvOverrides(cfg)
	return cfg, nil
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
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
}
