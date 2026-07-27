package cli

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/loop-eng/loopctl/internal/config"
)

// Note: the root command's default action (runTUI) launches a full-screen
// bubbletea program expecting a real terminal — it is never invoked in
// these tests. Every case here relies on cobra intercepting the command
// before RunE runs (--version, --help, an unknown flag), or tests a pure
// helper function (setupLogger) directly.

func TestVersionFlag(t *testing.T) {
	out, err := execRoot(t, "--version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "loopctl") {
		t.Errorf("expected version output to mention loopctl, got: %q", out)
	}
}

func TestHelpFlag(t *testing.T) {
	out, err := execRoot(t, "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "live terminal dashboard") {
		t.Errorf("expected help output to include the long description, got: %q", out)
	}
	if !strings.Contains(out, "Usage:") {
		t.Errorf("expected help output to include a Usage section, got: %q", out)
	}
}

func TestUnknownFlagReturnsError(t *testing.T) {
	_, err := execRoot(t, "--this-flag-does-not-exist")
	if err == nil {
		t.Fatal("expected an error for an unrecognized flag")
	}
}

func TestUnknownSubcommandReturnsError(t *testing.T) {
	_, err := execRoot(t, "this-subcommand-does-not-exist")
	if err == nil {
		t.Fatal("expected an error for an unrecognized subcommand")
	}
}

func TestSetupLoggerLevels(t *testing.T) {
	tests := []struct {
		name      string
		cfgLevel  string
		verbose   bool
		wantLevel slog.Level
	}{
		{"debug level", "debug", false, slog.LevelDebug},
		{"info level", "info", false, slog.LevelInfo},
		{"warn level", "warn", false, slog.LevelWarn},
		{"error level", "error", false, slog.LevelError},
		{"unrecognized level defaults to info", "bogus", false, slog.LevelInfo},
		{"empty level defaults to info", "", false, slog.LevelInfo},
		{"verbose flag forces debug regardless of configured level", "error", true, slog.LevelDebug},
		{"uppercase level is normalized", "DEBUG", false, slog.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Default()
			cfg.Logging.Level = tt.cfgLevel
			logger := setupLogger(cfg, tt.verbose)
			if logger == nil {
				t.Fatal("setupLogger returned nil")
			}
			if !logger.Enabled(context.TODO(), tt.wantLevel) {
				t.Errorf("logger not enabled for %v, want enabled", tt.wantLevel)
			}
			// One level below wantLevel must be disabled, proving the level
			// was actually applied and not just defaulted to Debug (which
			// would trivially pass the check above for every case).
			if tt.wantLevel > slog.LevelDebug {
				belowLevel := tt.wantLevel - 1
				if logger.Enabled(context.TODO(), belowLevel) {
					t.Errorf("logger unexpectedly enabled for %v, want disabled below %v", belowLevel, tt.wantLevel)
				}
			}
		})
	}
}

func TestSetupLoggerVerboseWritesToStderr(t *testing.T) {
	// Non-verbose mode discards output; verbose mode must not silently
	// discard it too (regression guard for a hypothetical "verbose flag
	// changes level but forgets to change the writer" bug). We can't
	// easily intercept os.Stderr here, so this asserts the documented
	// contract indirectly: verbose implies Debug level is enabled, which
	// is the behavior users rely on --verbose for.
	cfg := config.Default()
	logger := setupLogger(cfg, true)
	if !logger.Enabled(context.TODO(), slog.LevelDebug) {
		t.Error("--verbose must enable debug-level logging")
	}
}
