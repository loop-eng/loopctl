package source

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestTracePathJoin(t *testing.T) {
	e := NewLTFEnricher(slog.Default())
	got := e.TracePath("/tmp/myproject")
	want := filepath.Join("/tmp/myproject", ".loop", "trace.ltf.jsonl")
	if got != want {
		t.Errorf("TracePath() = %q, want %q", got, want)
	}
}

func TestAvailableTrueWhenFileExists(t *testing.T) {
	dir := t.TempDir()
	loopDir := filepath.Join(dir, ".loop")
	if err := os.MkdirAll(loopDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(loopDir, "trace.ltf.jsonl"), []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	e := NewLTFEnricher(slog.Default())
	if !e.Available(dir) {
		t.Error("expected Available=true when trace file exists")
	}
}

func TestAvailableFalseWhenMissing(t *testing.T) {
	dir := t.TempDir()
	e := NewLTFEnricher(slog.Default())
	if e.Available(dir) {
		t.Error("expected Available=false when trace file is missing")
	}
}

func TestAvailableFalseWhenSymlink(t *testing.T) {
	dir := t.TempDir()
	loopDir := filepath.Join(dir, ".loop")
	if err := os.MkdirAll(loopDir, 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "real-target.jsonl")
	if err := os.WriteFile(target, []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(loopDir, "trace.ltf.jsonl")); err != nil {
		t.Skipf("symlinks not supported on this filesystem: %v", err)
	}

	e := NewLTFEnricher(slog.Default())
	if e.Available(dir) {
		t.Error("expected Available=false for a symlinked trace file")
	}
}

func TestAvailableFalseForEmptyProjectDir(t *testing.T) {
	e := NewLTFEnricher(slog.Default())
	if e.Available("") {
		t.Error("expected Available=false for empty project dir")
	}
}
