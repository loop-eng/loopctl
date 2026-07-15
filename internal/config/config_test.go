package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()

	if cfg.Budget.PerSessionUSD != 20.0 {
		t.Errorf("per session = %.2f, want 20.0", cfg.Budget.PerSessionUSD)
	}
	if cfg.Budget.PerDayUSD != 200.0 {
		t.Errorf("per day = %.2f, want 200.0", cfg.Budget.PerDayUSD)
	}
	if cfg.Budget.WarnAtPercent != 80 {
		t.Errorf("warn at = %d, want 80", cfg.Budget.WarnAtPercent)
	}
	if cfg.Spin.RepeatedCalls != 3 {
		t.Errorf("repeated calls = %d, want 3", cfg.Spin.RepeatedCalls)
	}
	if cfg.Sources.ClaudeCode != "auto" {
		t.Errorf("claude code source = %q, want auto", cfg.Sources.ClaudeCode)
	}
	if cfg.RefreshRate != "1s" {
		t.Errorf("refresh rate = %q, want 1s", cfg.RefreshRate)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("loading missing file should not error: %v", err)
	}
	if cfg.Budget.PerSessionUSD != 20.0 {
		t.Error("missing file should return defaults")
	}
}

func TestLoadYAMLFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte(`
budget:
  per_session_usd: 50.0
  per_day_usd: 500.0
spin:
  repeated_calls: 5
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Budget.PerSessionUSD != 50.0 {
		t.Errorf("per session = %.2f, want 50.0", cfg.Budget.PerSessionUSD)
	}
	if cfg.Budget.PerDayUSD != 500.0 {
		t.Errorf("per day = %.2f, want 500.0", cfg.Budget.PerDayUSD)
	}
	if cfg.Spin.RepeatedCalls != 5 {
		t.Errorf("repeated calls = %d, want 5", cfg.Spin.RepeatedCalls)
	}
	if cfg.Budget.WarnAtPercent != 80 {
		t.Error("defaults should be applied for unset fields")
	}
}

func TestEnvOverrides(t *testing.T) {
	t.Setenv("LOOPCTL_BUDGET_PER_SESSION", "99.99")
	t.Setenv("LOOPCTL_LOG_LEVEL", "debug")

	cfg, err := Load("/nonexistent/path")
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Budget.PerSessionUSD != 99.99 {
		t.Errorf("env override per session = %.2f, want 99.99", cfg.Budget.PerSessionUSD)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("env override log level = %q, want debug", cfg.Logging.Level)
	}
}

func TestLoadTooLargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.yaml")
	data := make([]byte, 2<<20)
	os.WriteFile(path, data, 0644)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for oversized config")
	}
}
