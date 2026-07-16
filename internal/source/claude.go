package source

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type ClaudeDiscoverer struct {
	logger  *slog.Logger
	baseDir string
}

func NewClaudeDiscoverer(logger *slog.Logger) *ClaudeDiscoverer {
	home, _ := os.UserHomeDir()
	return &ClaudeDiscoverer{
		logger:  logger,
		baseDir: filepath.Join(home, ".claude", "projects"),
	}
}

func (d *ClaudeDiscoverer) Agent() string    { return "claude" }
func (d *ClaudeDiscoverer) BasePath() string { return d.baseDir }

func (d *ClaudeDiscoverer) Discover(maxAge time.Duration) []*Session {
	cutoff := time.Now().Add(-maxAge)
	var sessions []*Session

	entries, err := os.ReadDir(d.baseDir)
	if err != nil {
		d.logger.Debug("claude projects dir not found", "path", d.baseDir, "error", err)
		return nil
	}

	for _, projDir := range entries {
		if !projDir.IsDir() {
			continue
		}
		projPath := filepath.Join(d.baseDir, projDir.Name())
		files, err := os.ReadDir(projPath)
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

			sessionID := strings.TrimSuffix(f.Name(), ".jsonl")
			fullPath := filepath.Join(projPath, f.Name())

			pid := findSessionPID(sessionID, fullPath)
			meta := readSessionMeta(fullPath)

			startedAt := info.ModTime()
			if meta.Timestamp != "" {
				if t, err := time.Parse(time.RFC3339Nano, meta.Timestamp); err == nil {
					startedAt = t
				}
			}

			projectDir := DecodeProjectDir(projDir.Name())
			if meta.Cwd != "" {
				projectDir = meta.Cwd
			}

			sessions = append(sessions, &Session{
				ID:         sessionID,
				Agent:      "claude",
				Path:       fullPath,
				ProjectDir: projectDir,
				PID:        pid,
				Active:     pid > 0,
				StartedAt:  startedAt,
				LastEvent:  info.ModTime(),
			})
		}
	}

	d.logger.Debug("claude discovery complete", "sessions", len(sessions))
	return sessions
}

type sessionMeta struct {
	Timestamp string `json:"timestamp"`
	Cwd       string `json:"cwd"`
}

func readSessionMeta(path string) sessionMeta {
	if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink != 0 {
		return sessionMeta{}
	}
	f, err := os.Open(path)
	if err != nil {
		return sessionMeta{}
	}
	defer f.Close()

	var meta sessionMeta
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4096), 64*1024)
	for i := 0; i < 20 && scanner.Scan(); i++ {
		var entry sessionMeta
		if json.Unmarshal(scanner.Bytes(), &entry) != nil {
			continue
		}
		if meta.Timestamp == "" && entry.Timestamp != "" {
			meta.Timestamp = entry.Timestamp
		}
		if meta.Cwd == "" && entry.Cwd != "" {
			meta.Cwd = entry.Cwd
		}
		if meta.Timestamp != "" && meta.Cwd != "" {
			break
		}
	}
	return meta
}

func DecodeProjectDir(encoded string) string {
	return "/" + strings.ReplaceAll(strings.TrimPrefix(encoded, "-"), "-", "/")
}

func findSessionPID(sessionID, jsonlPath string) int {
	if pid := pgrepSessionID(sessionID); pid > 0 {
		return pid
	}
	if pid := lsofFile(jsonlPath); pid > 0 {
		return pid
	}
	return 0
}

func pgrepSessionID(sessionID string) int {
	out, err := exec.Command("pgrep", "-f", regexp.QuoteMeta(sessionID)).Output()
	if err != nil {
		return 0
	}
	return firstPID(out)
}

func lsofFile(path string) int {
	out, err := exec.Command("lsof", "-t", path).Output()
	if err != nil {
		return 0
	}
	return firstPID(out)
}

func firstPID(output []byte) int {
	lines := strings.Fields(strings.TrimSpace(string(output)))
	if len(lines) == 0 {
		return 0
	}
	var pid int
	fmt.Sscanf(lines[0], "%d", &pid)
	return pid
}
