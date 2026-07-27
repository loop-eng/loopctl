package app

import (
	"testing"

	"github.com/loop-eng/loopctl/internal/model"
)

func TestClassifyOutcome_Killed_TakesPriorityOverErrors(t *testing.T) {
	s := model.SessionView{SessionID: "s1", ErrorCount: 5}
	killed := map[string]bool{"s1": true}
	if got := classifyOutcome(s, killed); got != outcomeKilled {
		t.Errorf("got %v, want %v", got, outcomeKilled)
	}
}

func TestClassifyOutcome_Failed_WhenErrorCountPositive(t *testing.T) {
	s := model.SessionView{SessionID: "s1", ErrorCount: 1}
	if got := classifyOutcome(s, nil); got != outcomeFailed {
		t.Errorf("got %v, want %v", got, outcomeFailed)
	}
}

func TestClassifyOutcome_Warned_WhenHasWarningsOrSpinning(t *testing.T) {
	s1 := model.SessionView{SessionID: "s1", HasWarnings: true}
	if got := classifyOutcome(s1, nil); got != outcomeWarned {
		t.Errorf("got %v, want %v", got, outcomeWarned)
	}
	s2 := model.SessionView{SessionID: "s2", IsSpinning: true}
	if got := classifyOutcome(s2, nil); got != outcomeWarned {
		t.Errorf("got %v, want %v", got, outcomeWarned)
	}
}

func TestClassifyOutcome_Done_Default(t *testing.T) {
	s := model.SessionView{SessionID: "s1"}
	if got := classifyOutcome(s, nil); got != outcomeDone {
		t.Errorf("got %v, want %v", got, outcomeDone)
	}
}
