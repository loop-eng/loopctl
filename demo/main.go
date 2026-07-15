package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

type claudeEntry struct {
	Type      string    `json:"type"`
	UUID      string    `json:"uuid"`
	RequestID string    `json:"requestId"`
	SessionID string    `json:"sessionId"`
	Timestamp string    `json:"timestamp"`
	Cwd       string    `json:"cwd,omitempty"`
	Message   claudeMsg `json:"message"`
}

type claudeMsg struct {
	Role    string           `json:"role"`
	Model   string           `json:"model,omitempty"`
	Content []claudeContent  `json:"content"`
	Usage   *claudeUsage     `json:"usage,omitempty"`
}

type claudeUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

type claudeContent struct {
	Type    string `json:"type"`
	Name    string `json:"name,omitempty"`
	Text    string `json:"text,omitempty"`
	Input   any    `json:"input,omitempty"`
	Content string `json:"content,omitempty"`
	IsError bool   `json:"is_error,omitempty"`
}

func main() {
	scenario := flag.String("scenario", "all", "Scenario: normal, spin-tool, spin-error, budget, stall, all")
	speed := flag.Float64("speed", 1.0, "Speed multiplier (higher = faster)")
	flag.Parse()

	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".claude", "projects", "-tmp-loopctl-demo")
	os.MkdirAll(baseDir, 0755)

	fmt.Println("LoopCtl Demo Simulator")
	fmt.Printf("Writing sessions to: %s\n", baseDir)
	fmt.Printf("Scenario: %s, Speed: %.1fx\n\n", *scenario, *speed)

	delay := func(d time.Duration) {
		time.Sleep(time.Duration(float64(d) / *speed))
	}

	switch *scenario {
	case "normal":
		runNormal(baseDir, delay)
	case "spin-tool":
		runSpinTool(baseDir, delay)
	case "spin-error":
		runSpinError(baseDir, delay)
	case "budget":
		runBudget(baseDir, delay)
	case "stall":
		runStall(baseDir, delay)
	case "all":
		fmt.Println("Starting all scenarios in parallel...")
		go runNormal(baseDir, delay)
		go runSpinTool(baseDir, delay)
		go runSpinError(baseDir, delay)
		go runBudget(baseDir, delay)
		runStall(baseDir, delay)
	default:
		fmt.Fprintf(os.Stderr, "Unknown scenario: %s\n", *scenario)
		os.Exit(1)
	}
}

func writeEntry(f *os.File, entry claudeEntry) {
	data, _ := json.Marshal(entry)
	f.Write(data)
	f.Write([]byte("\n"))
}

func assistantToolUse(sessionID, model, tool string, input any, inputTok, outputTok, cacheRead, cacheWrite int) claudeEntry {
	return claudeEntry{
		Type:      "assistant",
		UUID:      fmt.Sprintf("u-%s-%d", sessionID[:8], rand.Intn(100000)),
		RequestID: fmt.Sprintf("r-%s-%d", sessionID[:8], rand.Intn(100000)),
		SessionID: sessionID,
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Message: claudeMsg{
			Role:  "assistant",
			Model: model,
			Content: []claudeContent{{
				Type:  "tool_use",
				Name:  tool,
				Input: input,
			}},
			Usage: &claudeUsage{
				InputTokens:              inputTok,
				OutputTokens:             outputTok,
				CacheReadInputTokens:     cacheRead,
				CacheCreationInputTokens: cacheWrite,
			},
		},
	}
}

func userToolResult(sessionID, content string, isError bool) claudeEntry {
	return claudeEntry{
		Type:      "user",
		UUID:      fmt.Sprintf("u-%s-%d", sessionID[:8], rand.Intn(100000)),
		SessionID: sessionID,
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Message: claudeMsg{
			Role: "user",
			Content: []claudeContent{{
				Type:    "tool_result",
				Content: content,
				IsError: isError,
			}},
		},
	}
}

func openSession(baseDir, sessionID string) *os.File {
	path := filepath.Join(baseDir, sessionID+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open %s: %v\n", path, err)
		os.Exit(1)
	}

	init := claudeEntry{
		Type:      "system",
		SessionID: sessionID,
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Cwd:       "/tmp/loopctl-demo",
		Message:   claudeMsg{Role: "system", Content: []claudeContent{{Type: "text", Text: "Session started"}}},
	}
	writeEntry(f, init)
	return f
}

func runNormal(baseDir string, delay func(time.Duration)) {
	sessionID := "demo-normal-session-001"
	f := openSession(baseDir, sessionID)
	defer f.Close()
	model := "claude-sonnet-4-5"

	fmt.Println("[normal] Starting normal coding session...")

	tools := []struct {
		name  string
		input any
	}{
		{"Read", map[string]any{"file_path": "/tmp/demo/main.go"}},
		{"Edit", map[string]any{"file_path": "/tmp/demo/main.go", "old_string": "func old()", "new_string": "func new()"}},
		{"Bash", map[string]any{"command": "go build ./..."}},
		{"Read", map[string]any{"file_path": "/tmp/demo/utils.go"}},
		{"Write", map[string]any{"file_path": "/tmp/demo/handler.go", "content": "package main"}},
		{"Bash", map[string]any{"command": "go test ./..."}},
		{"Edit", map[string]any{"file_path": "/tmp/demo/handler.go", "old_string": "return nil", "new_string": "return err"}},
		{"Bash", map[string]any{"command": "go test -race ./..."}},
	}

	for i, t := range tools {
		writeEntry(f, assistantToolUse(sessionID, model, t.name, t.input, 5000+i*1000, 2000+i*500, 3000, 500))
		delay(2 * time.Second)
		writeEntry(f, userToolResult(sessionID, "ok", false))
		delay(1 * time.Second)
	}
	fmt.Println("[normal] Done — session completed normally")
}

func runSpinTool(baseDir string, delay func(time.Duration)) {
	sessionID := "demo-spin-tool-session-002"
	f := openSession(baseDir, sessionID)
	defer f.Close()
	model := "claude-opus-4-6"

	fmt.Println("[spin-tool] Starting spinning session (repeated tool calls)...")

	input := map[string]any{"file_path": "/tmp/demo/broken.go", "old_string": "old", "new_string": "new"}

	for i := 0; i < 8; i++ {
		writeEntry(f, assistantToolUse(sessionID, model, "Edit", input, 8000, 4000, 5000, 1000))
		delay(2 * time.Second)
		writeEntry(f, userToolResult(sessionID, "ok", false))
		delay(1 * time.Second)
	}
	fmt.Println("[spin-tool] Done — should have triggered spin detection after 3 identical calls")
}

func runSpinError(baseDir string, delay func(time.Duration)) {
	sessionID := "demo-spin-error-session-003"
	f := openSession(baseDir, sessionID)
	defer f.Close()
	model := "claude-opus-4-6"

	fmt.Println("[spin-error] Starting error echo session...")

	for i := 0; i < 6; i++ {
		writeEntry(f, assistantToolUse(sessionID, model, "Bash", map[string]any{"command": "npm test"}, 6000, 3000, 4000, 800))
		delay(2 * time.Second)
		writeEntry(f, userToolResult(sessionID, "Error: Cannot find module 'express'", true))
		delay(1 * time.Second)
	}
	fmt.Println("[spin-error] Done — should have triggered error echo after 3 identical errors")
}

func runBudget(baseDir string, delay func(time.Duration)) {
	sessionID := "demo-budget-session-004"
	f := openSession(baseDir, sessionID)
	defer f.Close()
	model := "claude-opus-4-6"

	fmt.Println("[budget] Starting high-cost session...")

	for i := 0; i < 10; i++ {
		writeEntry(f, assistantToolUse(sessionID, model, "Bash",
			map[string]any{"command": fmt.Sprintf("iteration %d", i)},
			50000, 25000, 10000, 5000))
		delay(2 * time.Second)
		writeEntry(f, userToolResult(sessionID, "ok", false))
		delay(1 * time.Second)
	}
	fmt.Println("[budget] Done — should show high cost in dashboard")
}

func runStall(baseDir string, delay func(time.Duration)) {
	sessionID := "demo-stall-session-005"
	f := openSession(baseDir, sessionID)
	defer f.Close()
	model := "claude-sonnet-4-5"

	fmt.Println("[stall] Starting stall session (reading but never editing)...")

	writeEntry(f, assistantToolUse(sessionID, model, "Edit", map[string]any{"file_path": "/tmp/demo/x.go"}, 5000, 2000, 3000, 500))
	delay(1 * time.Second)
	writeEntry(f, userToolResult(sessionID, "ok", false))

	for i := 0; i < 10; i++ {
		writeEntry(f, assistantToolUse(sessionID, model, "Read",
			map[string]any{"file_path": fmt.Sprintf("/tmp/demo/file%d.go", i)},
			4000+i*100, 1000, 3000, 200))
		delay(3 * time.Second)
		writeEntry(f, userToolResult(sessionID, "file contents...", false))
		delay(2 * time.Second)
	}
	fmt.Println("[stall] Done — should show stall warning (reads but no edits)")
}
