package panel

import (
	"strings"
	"testing"
	"time"

	"github.com/loop-eng/loopctl/internal/model"
)

func TestHistoryPanel_Update_SortsMostRecentFirst(t *testing.T) {
	p := NewHistoryPanel()
	now := time.Now()

	sessions := []model.SessionView{
		{SessionID: "old", LastActivity: now.Add(-2 * time.Hour)},
		{SessionID: "newest", LastActivity: now},
		{SessionID: "mid", LastActivity: now.Add(-1 * time.Hour)},
	}
	p.Update(sessions, map[string]string{})

	if len(p.sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(p.sessions))
	}
	if p.sessions[0].SessionID != "newest" || p.sessions[1].SessionID != "mid" || p.sessions[2].SessionID != "old" {
		t.Errorf("expected most-recent-first order, got %v", []string{p.sessions[0].SessionID, p.sessions[1].SessionID, p.sessions[2].SessionID})
	}
}

func TestHistoryToRow_FormatsEndedColumn(t *testing.T) {
	zero := historyToRow(model.SessionView{}, "Done")
	if zero[2] != "—" {
		t.Errorf("expected '—' for zero-value LastActivity, got %q", zero[2])
	}

	recent := historyToRow(model.SessionView{LastActivity: time.Now().Add(-5 * time.Minute)}, "Done")
	if !strings.Contains(recent[2], "m ago") {
		t.Errorf("expected minutes-ago format, got %q", recent[2])
	}
}

func TestHistoryToRow_UsesLTFIterationCountWhenAvailable(t *testing.T) {
	s := model.SessionView{IterationCount: 5, LTFAvailable: true, LTFIterationCount: 2}
	row := historyToRow(s, "Done")
	if row[4] != "2" {
		t.Errorf("expected LTF iteration count 2, got %q", row[4])
	}
}

func TestHumanizeRelative(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{48 * time.Hour, "2d ago"},
	}
	for _, tc := range cases {
		if got := humanizeRelative(tc.d); got != tc.want {
			t.Errorf("humanizeRelative(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestHistoryPanel_Summary_EmptyNoPanic(t *testing.T) {
	p := NewHistoryPanel()
	p.SetSize(80, 20)
	got := p.Summary(map[string]string{})
	if !strings.Contains(got, "No completed sessions yet") {
		t.Errorf("expected empty-state message, got %q", got)
	}
}

func TestHistoryPanel_Summary_CountsByOutcome(t *testing.T) {
	p := NewHistoryPanel()
	p.SetSize(80, 20)
	sessions := []model.SessionView{
		{SessionID: "a", TotalCost: 1.0},
		{SessionID: "b", TotalCost: 2.0},
	}
	outcomes := map[string]string{"a": "Done", "b": "Failed"}
	p.Update(sessions, outcomes)

	got := p.Summary(outcomes)
	if !strings.Contains(got, "2 sessions") || !strings.Contains(got, "$3.00 total") {
		t.Errorf("expected summary counts, got %q", got)
	}
}
