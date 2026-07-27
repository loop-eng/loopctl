package cli

import (
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/loop-eng/loopctl/internal/app"
	"github.com/loop-eng/loopctl/internal/config"
	"github.com/loop-eng/loopctl/internal/source"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "loopctl",
	Short: "htop for AI coding agents",
	Long: `LoopCtl is a live terminal dashboard for AI agent sessions.
Monitor Claude Code and Codex CLI sessions with real-time
cost tracking, context health, and spin detection.`,
	Version:       version + " (" + commit + ") " + date,
	RunE:          runTUI,
	SilenceErrors: true,
	SilenceUsage:  true,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file (default: ~/.config/loopctl/config.yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "enable debug logging")
}

func runTUI(cmd *cobra.Command, args []string) error {
	cfgPath, _ := cmd.Flags().GetString("config")
	cfg, issues, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	for _, issue := range issues {
		cmd.PrintErrf("[%s] %s: %s\n", issue.Severity, issue.Field, issue.Message)
	}

	verbose, _ := cmd.Flags().GetBool("verbose")
	logger := setupLogger(cfg, verbose)

	refreshRate, err := time.ParseDuration(cfg.RefreshRate)
	if err != nil {
		refreshRate = time.Second
	}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	collector := source.NewCollector(logger, cfg)
	collector.Start(ctx)
	defer collector.Close()

	model := app.New(collector, refreshRate)

	p := tea.NewProgram(model)
	_, err = p.Run()
	return err
}

func setupLogger(cfg *config.Config, verbose bool) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.Logging.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	if verbose {
		level = slog.LevelDebug
	}

	w := io.Discard
	if verbose {
		w = os.Stderr
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}
