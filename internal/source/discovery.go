package source

import (
	"sync"
	"time"
)

type Session struct {
	ID         string
	Agent      string
	Path       string
	ProjectDir string
	PID        int
	Active     bool
	StartedAt  time.Time
	LastEvent  time.Time
}

type Discoverer interface {
	Agent() string
	BasePath() string
	Discover(maxAge time.Duration) []*Session
}

type Registry struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

func NewRegistry() *Registry {
	return &Registry{
		sessions: make(map[string]*Session),
	}
}

func (r *Registry) Add(s *Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.ID] = s
}

func (r *Registry) TryAdd(s *Session) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.sessions[s.ID]; exists {
		return false
	}
	r.sessions[s.ID] = s
	return true
}

func (r *Registry) Get(id string) (Session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	if !ok {
		return Session{}, false
	}
	return *s, true
}

func (r *Registry) Update(id string, fn func(*Session)) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sessions[id]
	if !ok {
		return false
	}
	fn(s)
	return true
}

func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, id)
}

func (r *Registry) Active() []Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var active []Session
	for _, s := range r.sessions {
		if s.Active {
			active = append(active, *s)
		}
	}
	return active
}

func (r *Registry) All() []Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	all := make([]Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		all = append(all, *s)
	}
	return all
}

func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}
