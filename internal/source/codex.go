package source

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CodexDiscoverer struct {
	logger  *slog.Logger
	baseDir string
}

func NewCodexDiscoverer(logger *slog.Logger) *CodexDiscoverer {
	home, _ := os.UserHomeDir()
	return &CodexDiscoverer{
		logger:  logger,
		baseDir: filepath.Join(home, ".codex", "sessions"),
	}
}

func (d *CodexDiscoverer) Agent() string    { return "codex" }
func (d *CodexDiscoverer) BasePath() string { return d.baseDir }

func (d *CodexDiscoverer) Discover(maxAge time.Duration) []*Session {
	cutoff := time.Now().Add(-maxAge)
	var sessions []*Session

	sessionDirs, err := os.ReadDir(d.baseDir)
	if err != nil {
		d.logger.Debug("codex sessions dir not found", "path", d.baseDir, "error", err)
		return nil
	}

	for _, dir := range sessionDirs {
		if !dir.IsDir() {
			continue
		}
		sessionPath := filepath.Join(d.baseDir, dir.Name())
		files, err := os.ReadDir(sessionPath)
		if err != nil {
			continue
		}

		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			info, err := f.Info()
			if err != nil || info.ModTime().Before(cutoff) {
				continue
			}

			fullPath := filepath.Join(sessionPath, f.Name())
			sessionID := dir.Name()

			pid := findSessionPID(sessionID, fullPath)
			sessions = append(sessions, &Session{
				ID:         sessionID,
				Agent:      "codex",
				Path:       fullPath,
				ProjectDir: sessionPath,
				PID:        pid,
				Active:     pid > 0,
				StartedAt:  info.ModTime(),
				LastEvent:  info.ModTime(),
			})
		}
	}

	d.logger.Debug("codex discovery complete", "sessions", len(sessions))
	return sessions
}
