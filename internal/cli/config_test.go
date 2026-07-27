package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/loop-eng/loopctl/internal/config"
)

// execRoot runs rootCmd with the given args, capturing stdout/stderr, and
// returns the combined output and the error from Execute().
func execRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	return buf.String(), err
}

func TestConfigInitCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if _, err := execRoot(t, "config", "init", "--config", path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	if string(data) != defaultConfigYAML {
		t.Error("written content does not match defaultConfigYAML")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected mode 0600, got %v", info.Mode().Perm())
	}
}

func TestConfigInitMissingParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "config.yaml")

	if _, err := execRoot(t, "config", "init", "--config", path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist in nested dir: %v", err)
	}
}

func TestConfigInitAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := []byte("original content\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}

	_, err := execRoot(t, "config", "init", "--config", path)
	if err == nil {
		t.Fatal("expected error when config already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != string(original) {
		t.Error("original file content should not be overwritten")
	}
}

func TestConfigInitHonorsConfigFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom-name.yaml")

	if _, err := execRoot(t, "config", "init", "--config", path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file at custom --config path: %v", err)
	}
	if _, err := os.Stat(config.DefaultPath()); err == nil {
		t.Fatal("should not have written to the default config path")
	}
}

func TestConfigInitRoundTripsWithLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if _, err := execRoot(t, "config", "init", "--config", path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, issues, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error loading generated config: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected no validation issues for the generated default config, got %v", issues)
	}
	if !reflect.DeepEqual(*cfg, *config.Default()) {
		t.Errorf("loaded config does not match config.Default(): %+v", cfg)
	}
}

func TestConfigRootCmdReportsStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	out, err := execRoot(t, "config", "--config", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No config file found") {
		t.Errorf("expected missing-file hint, got: %q", out)
	}

	if _, err := execRoot(t, "config", "init", "--config", path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err = execRoot(t, "config", "--config", path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "No config file found") {
		t.Errorf("expected no missing-file hint once the file exists, got: %q", out)
	}
}

func TestConfigValidateCommandCleanConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if _, err := execRoot(t, "config", "init", "--config", path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := execRoot(t, "config", "validate", "--config", path)
	if err != nil {
		t.Fatalf("unexpected error for a clean config: %v", err)
	}
	if !strings.Contains(out, "OK") {
		t.Errorf("expected OK message, got: %q", out)
	}
}

func TestConfigValidateCommandReportsIssues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "sources:\n  codex: dissabled\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	out, err := execRoot(t, "config", "validate", "--config", path)
	if err == nil {
		t.Fatal("expected a non-nil error when issues are found")
	}
	if !strings.Contains(out, "sources.codex") {
		t.Errorf("expected the issue field to be reported, got: %q", out)
	}
}

func TestConfigValidateCommandMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.yaml")

	out, err := execRoot(t, "config", "validate", "--config", path)
	if err != nil {
		t.Fatalf("missing file should not be an error: %v", err)
	}
	if !strings.Contains(out, "defaults are in effect") {
		t.Errorf("expected defaults-in-effect message, got: %q", out)
	}
}
