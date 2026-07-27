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
	cfg, issues, err := Load("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("loading missing file should not error: %v", err)
	}
	if cfg.Budget.PerSessionUSD != 20.0 {
		t.Error("missing file should return defaults")
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues for defaults, got %v", issues)
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

	cfg, issues, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues for a valid config, got %v", issues)
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

	cfg, _, err := Load("/nonexistent/path")
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

func TestLoadOversizedFileStillHardErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.yaml")
	data := make([]byte, 2<<20)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := Load(path)
	if err == nil {
		t.Fatal("expected error for oversized config")
	}
}

func TestLoadMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// Unbalanced flow mapping — invalid YAML syntax.
	content := []byte("budget: {per_session_usd: 50.0\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg, issues, err := Load(path)
	if err != nil {
		t.Fatalf("malformed YAML should not hard-error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected a usable config even on parse failure")
	}
	if cfg.Budget.PerSessionUSD != Default().Budget.PerSessionUSD {
		t.Errorf("expected defaults after parse failure, got per_session_usd=%.2f", cfg.Budget.PerSessionUSD)
	}

	found := false
	for _, iss := range issues {
		if iss.Severity == "error" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an error-severity issue for malformed YAML, got %v", issues)
	}
}

func TestValidateSourcesEnumInvalid(t *testing.T) {
	cfg := Default()
	cfg.Sources.Codex = "dissabled"

	issues := Validate(cfg)

	if cfg.Sources.Codex != "auto" {
		t.Errorf("invalid codex source should reset to auto, got %q", cfg.Sources.Codex)
	}
	if !hasField(issues, "sources.codex") {
		t.Errorf("expected an issue for sources.codex, got %v", issues)
	}
}

func TestValidateWarnAtPercentOutOfRange(t *testing.T) {
	cfg := Default()
	cfg.Budget.WarnAtPercent = 800

	issues := Validate(cfg)

	if cfg.Budget.WarnAtPercent != 80 {
		t.Errorf("out-of-range warn_at_percent should clamp to 80, got %d", cfg.Budget.WarnAtPercent)
	}
	if !hasField(issues, "budget.warn_at_percent") {
		t.Errorf("expected an issue for budget.warn_at_percent, got %v", issues)
	}
}

func TestValidateNegativeBudget(t *testing.T) {
	cfg := Default()
	cfg.Budget.PerSessionUSD = -5

	issues := Validate(cfg)

	if cfg.Budget.PerSessionUSD != 0 {
		t.Errorf("negative budget should clamp to 0, got %.2f", cfg.Budget.PerSessionUSD)
	}
	if !hasField(issues, "budget.per_session_usd") {
		t.Errorf("expected an issue for budget.per_session_usd, got %v", issues)
	}
}

func TestValidateLoggingLevelInvalid(t *testing.T) {
	cfg := Default()
	cfg.Logging.Level = "verbose"

	issues := Validate(cfg)

	if cfg.Logging.Level != "info" {
		t.Errorf("invalid log level should reset to info, got %q", cfg.Logging.Level)
	}
	if !hasField(issues, "logging.level") {
		t.Errorf("expected an issue for logging.level, got %v", issues)
	}
}

func TestValidateRefreshRateInvalid(t *testing.T) {
	cfg := Default()
	cfg.RefreshRate = "not-a-duration"

	issues := Validate(cfg)

	if cfg.RefreshRate != "1s" {
		t.Errorf("invalid refresh_rate should reset to 1s, got %q", cfg.RefreshRate)
	}
	if !hasField(issues, "refresh_rate") {
		t.Errorf("expected an issue for refresh_rate, got %v", issues)
	}
}

func TestValidateRefreshRateTooFast(t *testing.T) {
	cfg := Default()
	cfg.RefreshRate = "1ms"

	issues := Validate(cfg)

	if cfg.RefreshRate != minRefreshRate.String() {
		t.Errorf("too-fast refresh_rate should clamp to floor, got %q", cfg.RefreshRate)
	}
	if !hasField(issues, "refresh_rate") {
		t.Errorf("expected an issue for refresh_rate, got %v", issues)
	}
}

func TestValidateCleanConfigNoIssues(t *testing.T) {
	cfg := Default()

	issues := Validate(cfg)

	if len(issues) != 0 {
		t.Errorf("expected no issues for a clean default config, got %v", issues)
	}
}

func TestValidateNegativeSpinFields(t *testing.T) {
	cfg := Default()
	cfg.Spin.StallMinutes = -1
	cfg.Spin.CostVelocityPerMin = -2.0

	issues := Validate(cfg)

	if cfg.Spin.StallMinutes != Default().Spin.StallMinutes {
		t.Errorf("negative stall_minutes should reset to default, got %d", cfg.Spin.StallMinutes)
	}
	if cfg.Spin.CostVelocityPerMin != Default().Spin.CostVelocityPerMin {
		t.Errorf("negative cost_velocity_per_min should reset to default, got %.2f", cfg.Spin.CostVelocityPerMin)
	}
	if !hasField(issues, "spin.stall_minutes") || !hasField(issues, "spin.cost_velocity_per_min") {
		t.Errorf("expected issues for both negative spin fields, got %v", issues)
	}
}

func hasField(issues []ValidationIssue, field string) bool {
	for _, iss := range issues {
		if iss.Field == field {
			return true
		}
	}
	return false
}
