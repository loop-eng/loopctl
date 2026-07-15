package source

import (
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r.Len() != 0 {
		t.Fatalf("expected empty registry, got %d", r.Len())
	}
}

func TestRegistryAdd(t *testing.T) {
	r := NewRegistry()
	s := &Session{ID: "abc", Agent: "claude", Active: true}
	r.Add(s)

	got, ok := r.Get("abc")
	if !ok {
		t.Fatal("expected session to exist")
	}
	if got.Agent != "claude" {
		t.Fatalf("expected agent=claude, got %s", got.Agent)
	}
}

func TestRegistryTryAdd(t *testing.T) {
	r := NewRegistry()
	s1 := &Session{ID: "abc", Agent: "claude"}
	s2 := &Session{ID: "abc", Agent: "codex"}

	if !r.TryAdd(s1) {
		t.Fatal("first TryAdd should succeed")
	}
	if r.TryAdd(s2) {
		t.Fatal("second TryAdd should fail")
	}

	got, _ := r.Get("abc")
	if got.Agent != "claude" {
		t.Fatalf("expected first session preserved, got agent=%s", got.Agent)
	}
}

func TestRegistryUpdate(t *testing.T) {
	r := NewRegistry()
	r.Add(&Session{ID: "abc", PID: 100})

	ok := r.Update("abc", func(s *Session) {
		s.PID = 200
	})
	if !ok {
		t.Fatal("update should succeed")
	}

	got, _ := r.Get("abc")
	if got.PID != 200 {
		t.Fatalf("expected PID=200, got %d", got.PID)
	}

	if r.Update("nonexistent", func(s *Session) {}) {
		t.Fatal("update of nonexistent should return false")
	}
}

func TestRegistryRemove(t *testing.T) {
	r := NewRegistry()
	r.Add(&Session{ID: "abc"})
	r.Remove("abc")

	if _, ok := r.Get("abc"); ok {
		t.Fatal("removed session should not exist")
	}
}

func TestRegistryActive(t *testing.T) {
	r := NewRegistry()
	r.Add(&Session{ID: "a", Active: true})
	r.Add(&Session{ID: "b", Active: false})
	r.Add(&Session{ID: "c", Active: true})

	active := r.Active()
	if len(active) != 2 {
		t.Fatalf("expected 2 active, got %d", len(active))
	}
}

func TestRegistryAll(t *testing.T) {
	r := NewRegistry()
	r.Add(&Session{ID: "a"})
	r.Add(&Session{ID: "b"})

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(all))
	}
}

func TestRegistryCopySemantics(t *testing.T) {
	r := NewRegistry()
	r.Add(&Session{ID: "abc", PID: 100})

	got, _ := r.Get("abc")
	got.PID = 999

	original, _ := r.Get("abc")
	if original.PID != 100 {
		t.Fatal("Get should return a copy, not a reference")
	}
}

func TestDecodeProjectDir(t *testing.T) {
	tests := []struct {
		encoded  string
		expected string
	}{
		{"-Users-foo-project", "/Users/foo/project"},
		{"-home-user-code-app", "/home/user/code/app"},
		{"-tmp", "/tmp"},
	}
	for _, tt := range tests {
		got := DecodeProjectDir(tt.encoded)
		if got != tt.expected {
			t.Errorf("DecodeProjectDir(%q) = %q, want %q", tt.encoded, got, tt.expected)
		}
	}
}

func TestSessionStruct(t *testing.T) {
	s := Session{
		ID:        "test-123",
		Agent:     "claude",
		Active:    true,
		PID:       42,
		StartedAt: time.Now(),
	}

	if s.ID != "test-123" || s.Agent != "claude" || !s.Active || s.PID != 42 {
		t.Fatal("session fields not set correctly")
	}
}
