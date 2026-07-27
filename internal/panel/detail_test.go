package panel

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/loop-eng/loopctl/internal/model"
)

func TestDetailPanel_SetSession_RendersAllSections(t *testing.T) {
	p := NewDetailPanel()
	p.SetSize(80, 24)

	s := model.SessionView{
		SessionID:        "abcdef1234567890",
		ProjectName:      "my-project",
		FilesChangedList: []string{"/tmp/foo.go", "/tmp/bar.go"},
		ErrorMessages:    []string{"error one", "error two"},
		SpinReasons:      []string{"same tool call repeated 3 times"},
		IsSpinning:       true,
	}
	p.SetSession(s)

	body := renderDetailBody(s, 80)
	for _, want := range []string{"/tmp/foo.go", "/tmp/bar.go", "error one", "error two", "same tool call repeated 3 times"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected detail body to contain %q, got:\n%s", want, body)
		}
	}
}

func TestDetailPanel_SetSession_EmptyLists(t *testing.T) {
	p := NewDetailPanel()
	p.SetSize(80, 24)

	s := model.SessionView{SessionID: "empty-session"}
	p.SetSession(s)

	body := renderDetailBody(s, 80)
	for _, want := range []string{"No files changed", "No errors", "No spin or stall warnings"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected empty-state text %q, got:\n%s", want, body)
		}
	}
}

func TestDetailPanel_ScrollKeys(t *testing.T) {
	p := NewDetailPanel()
	p.SetSize(40, 10)

	var files []string
	for i := 0; i < 50; i++ {
		files = append(files, "/tmp/file.go")
	}
	s := model.SessionView{SessionID: "s1", FilesChangedList: files, FilesChanged: len(files)}
	p.SetSession(s)

	msg := tea.KeyPressMsg{Code: tea.KeyDown}
	for i := 0; i < 5; i++ {
		var cmd tea.Cmd
		p, cmd = p.Update(msg)
		_ = cmd
	}
	// No panic and the panel remains usable — exact scroll offset depends on
	// bubbles/v2 viewport internals, so just confirm View() still renders.
	if p.View() == "" {
		t.Error("expected non-empty view after scrolling")
	}
}

func TestFormatInt_ThousandsSeparators(t *testing.T) {
	cases := map[int]string{
		0:       "0",
		5:       "5",
		999:     "999",
		1000:    "1,000",
		1234567: "1,234,567",
		-1234:   "-1,234",
	}
	for in, want := range cases {
		if got := formatInt(in); got != want {
			t.Errorf("formatInt(%d) = %q, want %q", in, got, want)
		}
	}
}
