package structures

import "sync"

type Set[T comparable] map[T]struct{}

func NewSet[T comparable](ts ...T) Set[T] {
	s := make(Set[T], len(ts))
	for _, t := range ts {
		s[t] = struct{}{}
	}
	return s
}

func (s Set[T]) Add(t T) { s[t] = struct{}{} }

func (s Set[T]) Remove(t T) { delete(s, t) }

func (s Set[T]) Range(f func(t T) bool) {
	for t := range s {
		if !f(t) {
			return
		}
	}
}

type SyncSet[T comparable] struct {
	sync.RWMutex
	Set[T]
}

func NewSyncSet[T comparable](ts ...T) *SyncSet[T] {
	s := &SyncSet[T]{Set: NewSet(ts...)}
	return s
}

func (s *SyncSet[T]) Add(t T) {
	s.Lock()
	defer s.Unlock()
	s.Set.Add(t)
}

func (s *SyncSet[T]) Remove(t T) {
	s.Lock()
	defer s.Unlock()
	s.Set.Remove(t)
}

func (s *SyncSet[T]) Range(f func(t T) bool) {
	s.RLock()
	defer s.RUnlock()
	s.Set.Range(f)
}
