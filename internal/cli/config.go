package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/loop-eng/loopctl/internal/config"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage LoopCtl configuration",
	Long: `Reports the LoopCtl configuration file location and whether it exists.
Default location: ~/.config/loopctl/config.yaml`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := resolveConfigPath(cmd)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			cmd.Printf("No config file found — using defaults.\n")
			cmd.Printf("Create one with: loopctl config init\n")
			cmd.Printf("Location: %s\n", path)
			return nil
		}
		cmd.Printf("Config file: %s\n", path)
		return nil
	},
}

const defaultConfigYAML = `# LoopCtl configuration
# All values shown are defaults — remove or modify as needed.

# How often the dashboard redraws. Accepts Go duration syntax (e.g. 500ms, 2s).
refresh_rate: 1s

budget:
  per_session_usd: 20.0    # sessions above this cost show a budget alert
  per_day_usd: 200.0       # daily cap across all sessions
  warn_at_percent: 80      # warn at this % of per_session_usd

spin:
  repeated_calls: 3         # same tool call N times in a row -> spin warning
  error_echo: 3              # same error N times -> spin warning
  stall_minutes: 10          # no activity for N min -> stall warning
  cost_velocity_per_min: 2.0 # $/min burn rate that triggers a warning

sources:
  claude_code: auto         # auto | disabled
  codex: auto                # auto | disabled
  custom: []                 # additional glob patterns to watch (reserved)

logging:
  level: info                # debug | info | warn | error

# Environment variable overrides:
#   LOOPCTL_BUDGET_PER_SESSION=30
#   LOOPCTL_BUDGET_PER_DAY=500
#   LOOPCTL_LOG_LEVEL=debug
`

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := resolveConfigPath(cmd)

		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("creating config dir %s: %w", dir, err)
		}

		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			if os.IsExist(err) {
				return fmt.Errorf("config already exists at %s (edit it directly, or remove it and re-run `loopctl config init`)", path)
			}
			return fmt.Errorf("creating config at %s: %w", path, err)
		}

		if _, err := f.WriteString(defaultConfigYAML); err != nil {
			f.Close()
			os.Remove(path)
			return fmt.Errorf("writing config: %w", err)
		}

		if err := f.Close(); err != nil {
			os.Remove(path)
			return fmt.Errorf("closing config file: %w", err)
		}

		cmd.Printf("Config created at %s\n", path)
		return nil
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the configuration file and report any issues",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := resolveConfigPath(cmd)

		if _, err := os.Stat(path); os.IsNotExist(err) {
			cmd.Printf("No config file at %s — defaults are in effect.\n", path)
			return nil
		}

		_, issues, err := config.Load(path)
		if err != nil {
			return err
		}
		if len(issues) == 0 {
			cmd.Println("OK — no issues found.")
			return nil
		}
		for _, issue := range issues {
			cmd.Printf("[%s] %s: %s\n", issue.Severity, issue.Field, issue.Message)
		}
		return fmt.Errorf("%d issue(s) found", len(issues))
	},
}

func resolveConfigPath(cmd *cobra.Command) string {
	path, _ := cmd.Flags().GetString("config")
	if path == "" {
		path = config.DefaultPath()
	}
	return path
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configValidateCmd)
	rootCmd.AddCommand(configCmd)
}
