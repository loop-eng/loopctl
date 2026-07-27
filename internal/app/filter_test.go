package app

import (
	"testing"

	"github.com/loop-eng/loopctl/internal/model"
)

func TestMatchesFilter_CaseInsensitiveSubstring(t *testing.T) {
	s := model.SessionView{ProjectName: "MyProject"}
	if !matchesFilter(s, "project") {
		t.Error("expected case-insensitive substring match")
	}
	if matchesFilter(s, "xyz") {
		t.Error("expected no match for unrelated substring")
	}
}

func TestMatchesFilter_EmptyQueryMatchesEverything(t *testing.T) {
	s := model.SessionView{ProjectName: "anything"}
	if !matchesFilter(s, "") {
		t.Error("empty query should match everything")
	}
}

func TestMatchesFilter_FallsBackToProjectDirBasename(t *testing.T) {
	s := model.SessionView{ProjectDir: "/Users/foo/my-app"}
	if !matchesFilter(s, "my-app") {
		t.Error("expected fallback to ProjectDir basename when ProjectName is empty")
	}
}

func TestApplyFilter_PreservesOrder(t *testing.T) {
	sessions := []model.SessionView{
		{ProjectName: "alpha"},
		{ProjectName: "beta"},
		{ProjectName: "alphabet"},
	}
	got := applyFilter(sessions, "alpha")
	if len(got) != 2 || got[0].ProjectName != "alpha" || got[1].ProjectName != "alphabet" {
		t.Errorf("unexpected filtered order: %v", got)
	}
}

func TestApplyFilter_NoMatches_ReturnsEmptyNotNil(t *testing.T) {
	sessions := []model.SessionView{{ProjectName: "alpha"}}
	got := applyFilter(sessions, "zzz")
	if got == nil {
		t.Error("expected non-nil empty slice")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 matches, got %d", len(got))
	}
}

func TestFilterActive_OnlyReturnsActiveTrue(t *testing.T) {
	sessions := []model.SessionView{
		{SessionID: "a", Active: true},
		{SessionID: "b", Active: false},
		{SessionID: "c", Active: true},
	}
	got := filterActive(sessions)
	if len(got) != 2 {
		t.Fatalf("expected 2 active sessions, got %d", len(got))
	}
	for _, s := range got {
		if !s.Active {
			t.Errorf("filterActive returned an inactive session: %s", s.SessionID)
		}
	}
}

func TestFilterCompleted_OnlyReturnsActiveFalse(t *testing.T) {
	sessions := []model.SessionView{
		{SessionID: "a", Active: true},
		{SessionID: "b", Active: false},
	}
	got := filterCompleted(sessions)
	if len(got) != 1 || got[0].SessionID != "b" {
		t.Errorf("expected only session b, got %v", got)
	}
}

func TestFilterActiveFilterCompleted_ArePartitionComplementary(t *testing.T) {
	sessions := []model.SessionView{
		{SessionID: "a", Active: true},
		{SessionID: "b", Active: false},
		{SessionID: "c", Active: true},
		{SessionID: "d", Active: false},
	}
	active := filterActive(sessions)
	completed := filterCompleted(sessions)
	if len(active)+len(completed) != len(sessions) {
		t.Fatalf("partition sizes don't add up: active=%d completed=%d total=%d",
			len(active), len(completed), len(sessions))
	}
	seen := make(map[string]bool)
	for _, s := range active {
		seen[s.SessionID] = true
	}
	for _, s := range completed {
		if seen[s.SessionID] {
			t.Errorf("session %s appeared in both partitions", s.SessionID)
		}
		seen[s.SessionID] = true
	}
	if len(seen) != len(sessions) {
		t.Errorf("expected every session in exactly one partition, got %d of %d", len(seen), len(sessions))
	}
}
