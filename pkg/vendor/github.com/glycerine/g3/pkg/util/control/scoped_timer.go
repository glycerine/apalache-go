package control

// Source: src/util/scoped_timer.h:24-30; src/util/scoped_timer.cpp:56-267.

import (
	"math"
	"sync"
	"time"
)

// EventHandler receives the source event-caller identifier.
type EventHandler interface {
	HandleEvent(EventCaller)
}

// EventHandlerFunc adapts a Go function to EventHandler.
type EventHandlerFunc func(EventCaller)

func (f EventHandlerFunc) HandleEvent(caller EventCaller) { f(caller) }

// ScopedTimerMemoryTrace observes the concrete source imp allocation lifetime.
// The caller owns the trace and supplies the process finalization guard.
type ScopedTimerMemoryTrace interface {
	AllocatedScopedTimer(*ScopedTimer)
	DeallocatedScopedTimer(*ScopedTimer)
}

// ScopedTimer is the Go timer counterpart used by util. A zero or MaxUint32
// interval is disabled, matching the source constructor. Close prevents a
// callback that has not begun; a callback already running owns its Go closure
// until it returns.
type ScopedTimer struct {
	self    *ScopedTimer
	mu      sync.Mutex
	timer   *time.Timer
	handler EventHandler
	trace   ScopedTimerMemoryTrace
	closed  bool
}

func NewScopedTimer(milliseconds uint32, handler EventHandler) (*ScopedTimer, error) {
	return NewScopedTimerWithMemoryTrace(milliseconds, handler, nil)
}

func NewScopedTimerWithMemoryTrace(milliseconds uint32, handler EventHandler, trace ScopedTimerMemoryTrace) (*ScopedTimer, error) {
	if handler == nil {
		return nil, ErrPrecondition
	}
	if f, ok := handler.(EventHandlerFunc); ok && f == nil {
		return nil, ErrPrecondition
	}
	s := &ScopedTimer{handler: handler, trace: trace}
	s.self = s
	if milliseconds == 0 || milliseconds == math.MaxUint32 {
		return s, nil
	}
	if s.trace != nil {
		s.trace.AllocatedScopedTimer(s)
	}
	s.timer = time.AfterFunc(time.Duration(milliseconds)*time.Millisecond, s.fire)
	return s, nil
}

func (s *ScopedTimer) fire() {
	s.mu.Lock()
	if s.closed || s.self != s {
		s.mu.Unlock()
		return
	}
	handler := s.handler
	s.mu.Unlock()
	handler.HandleEvent(EventCallerTimeout)
}

func (s *ScopedTimer) Close() error {
	if s == nil || s.self != s {
		return ErrOwner
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	trace := s.trace
	hadImp := s.timer != nil
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	s.handler = nil
	s.trace = nil
	s.mu.Unlock()
	if hadImp && trace != nil {
		trace.DeallocatedScopedTimer(s)
	}
	return nil
}

func (s *ScopedTimer) Closed() bool {
	if s == nil || s.self != s {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}
