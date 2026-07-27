package app

import (
	"path/filepath"
	"strings"

	"github.com/loop-eng/loopctl/internal/model"
)

// matchesFilter reports whether s's project name contains query,
// case-insensitively. An empty query matches everything.
func matchesFilter(s model.SessionView, query string) bool {
	if query == "" {
		return true
	}
	name := s.ProjectName
	if name == "" {
		name = filepath.Base(s.ProjectDir)
	}
	return strings.Contains(strings.ToLower(name), strings.ToLower(query))
}

// applyFilter returns the subset of sessions matching query, preserving
// order. Never returns nil (returns an empty, non-nil slice on no matches).
func applyFilter(sessions []model.SessionView, query string) []model.SessionView {
	out := make([]model.SessionView, 0, len(sessions))
	if query == "" {
		out = append(out, sessions...)
		return out
	}
	for _, s := range sessions {
		if matchesFilter(s, query) {
			out = append(out, s)
		}
	}
	return out
}

// filterActive returns only sessions with Active == true, preserving order.
func filterActive(sessions []model.SessionView) []model.SessionView {
	out := make([]model.SessionView, 0, len(sessions))
	for _, s := range sessions {
		if s.Active {
			out = append(out, s)
		}
	}
	return out
}

// filterCompleted returns only sessions with Active == false, preserving order.
func filterCompleted(sessions []model.SessionView) []model.SessionView {
	out := make([]model.SessionView, 0, len(sessions))
	for _, s := range sessions {
		if !s.Active {
			out = append(out, s)
		}
	}
	return out
}
