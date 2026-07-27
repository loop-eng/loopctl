package source

import (
	"log/slog"
	"os"
	"path/filepath"
)

// LTFEnricher locates the per-project LTF trace file (written by the
// opt-in Claude Code adapter at github.com/loop-eng/ltf/adapters/claude-code)
// for a given project directory. It does not implement Discoverer: there is
// no global LTF root to scan the way there is for ~/.claude/projects or
// ~/.codex/sessions — traces live inside arbitrary project directories,
// keyed by the same ProjectDir the native discoverers already resolve.
type LTFEnricher struct {
	logger *slog.Logger
}

func NewLTFEnricher(logger *slog.Logger) *LTFEnricher {
	return &LTFEnricher{logger: logger}
}

// TracePath returns the expected LTF trace path for a project directory.
// It does not check existence.
func (e *LTFEnricher) TracePath(projectDir string) string {
	return filepath.Join(projectDir, ".loop", "trace.ltf.jsonl")
}

// Available reports whether a readable, non-symlink trace file exists for
// projectDir. Mirrors the symlink-avoidance convention already used by
// Tailer.ReadNewLines and readSessionMeta — don't follow a symlink an
// attacker could swap to point outside the project.
func (e *LTFEnricher) Available(projectDir string) bool {
	if projectDir == "" {
		return false
	}
	path := e.TracePath(projectDir)
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	return info.Mode().IsRegular()
}
