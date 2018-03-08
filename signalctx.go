package golet

import (
	"context"
	"os"
	"sync"
	"time"
)

type signalCtx struct {
	parent  context.Context
	recv    chan struct{} // closed by coming signals.
	mu      sync.Mutex
	signal  os.Signal
	sigchan chan os.Signal
}

func (s *signalCtx) notifySignal() {
	if s.recv != nil {
		s.mu.Lock()
		close(s.recv) // Notify that signals has been received
		s.recv = make(chan struct{})
		s.mu.Unlock()
	}
}

// Recv send channel when a process receives a signal
func (s *signalCtx) Recv() <-chan struct{} {
	s.mu.Lock()
	if s.recv == nil {
		s.recv = make(chan struct{})
	}
	r := s.recv
	s.mu.Unlock()
	return r
}

// Signal returns os.Signal and error.
func (s *signalCtx) Signal() (os.Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.signal, nil
}

/* They are methods for context.Context */

// Deadline is implemented for context.Context
func (s *signalCtx) Deadline() (deadline time.Time, ok bool) {
	return s.parent.Deadline()
}

// Done is implemented for context.Context
func (s *signalCtx) Done() <-chan struct{} {
	return s.parent.Done()
}

// Err is implemented for context.Context
func (s *signalCtx) Err() error {
	return s.parent.Err()
}

// Value is implemented for context.Context
func (s *signalCtx) Value(key interface{}) interface{} {
	return s.parent.Value(key)
}
