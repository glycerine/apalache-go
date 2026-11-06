package shell

// Source: src/util/timeout.cpp:23-60; src/util/timeout.h:22-24.

import (
	"io"
	"os"
	"sync"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// TimeoutService owns the source shell's one replaceable process timer. Its
// callback and output stream are borrowed. Go cannot throw a C++ exception
// from a timer thread into the shell, so Events delivers that error explicitly.
type TimeoutService struct {
	self     *TimeoutService
	mu       sync.Mutex
	fireMu   sync.Mutex
	timer    *control.ScopedTimer
	callback func()
	output   io.Writer
	trace    *diagnostic.TraceTags
	events   chan error
	serial   uint64
	caller   control.EventCaller
	closed   bool
}

// The scoped timer owns this concrete source g_timeout_eh counterpart until
// replacement, expiry, or close; it borrows the process shell owner.
type timeoutEventHandler struct {
	owner  *TimeoutService
	serial uint64
}

// Source timeout.cpp publishes one process timer and one borrowed callback.
// Explicit test owners remain available through NewTimeoutService.
var processTimeout = NewTimeoutService(os.Stdout)

func BorrowProcessTimeoutService() *TimeoutService { return processTimeout }

func (h *timeoutEventHandler) HandleEvent(caller control.EventCaller) {
	h.owner.fire(h.serial, caller)
}

func NewTimeoutService(output io.Writer) *TimeoutService {
	if output == nil {
		output = os.Stdout
	}
	s := &TimeoutService{output: output, events: make(chan error, 1)}
	s.self = s
	return s
}

// RegisterOnTimeoutProc replaces the borrowed source function pointer.
func (s *TimeoutService) RegisterOnTimeoutProc(callback func()) error {
	if s == nil || s.self != s {
		return control.ErrOwner
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return control.ErrOwner
	}
	s.callback = callback
	return nil
}

// SetTimeout replaces the owned scoped_timer. C++ narrows long to unsigned at
// the constructor call; zero and UINT_MAX disable the timer.
func (s *TimeoutService) SetTimeout(milliseconds int64) error {
	if s == nil || s.self != s {
		return control.ErrOwner
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return control.ErrOwner
	}
	if s.timer != nil {
		if err := s.timer.Close(); err != nil {
			return err
		}
		s.timer = nil
	}
	s.serial++
	serial := s.serial
	timer, err := control.NewScopedTimerWithMemoryTrace(uint32(milliseconds), &timeoutEventHandler{owner: s, serial: serial}, newTimeoutMemoryTrace(s.trace))
	if err != nil {
		return err
	}
	s.timer = timer
	return nil
}

func (s *TimeoutService) fire(serial uint64, caller control.EventCaller) {
	// The source event handler holds g_timeout_cs through the borrowed
	// callback and deletion of the current global timer. A callback may
	// rearm that timer, but its event must wait until this handler finishes.
	s.fireMu.Lock()
	defer s.fireMu.Unlock()
	s.mu.Lock()
	if s.closed || serial != s.serial || s.timer == nil {
		s.mu.Unlock()
		return
	}
	output := s.output
	s.mu.Unlock()
	_, _ = io.WriteString(output, "timeout\n")
	s.mu.Lock()
	if s.closed || serial != s.serial || s.timer == nil {
		s.mu.Unlock()
		return
	}
	s.caller = caller
	callback := s.callback
	s.mu.Unlock()
	if callback != nil {
		callback()
	}
	s.mu.Lock()
	if !s.closed && s.timer != nil {
		_ = s.timer.Close()
		s.timer = nil
	}
	if !s.closed {
		// An uncaught source exception ends this shell path. Preserve its first
		// pending error if a caller rearms without consuming it; never strand
		// the timer callback on a full delivery channel.
		select {
		case s.events <- z3exception.MustZ3Error(z3exception.ErrorTimeout):
		default:
		}
	}
	s.mu.Unlock()
}

// Events is the shell-side transport for the source timeout exception.
func (s *TimeoutService) Events() <-chan error {
	if s == nil || s.self != s {
		return nil
	}
	return s.events
}

func (s *TimeoutService) Caller() control.EventCaller {
	if s == nil || s.self != s {
		return control.EventCallerUnset
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.caller
}

func (s *TimeoutService) Close() error {
	if s == nil || s.self != s {
		return control.ErrOwner
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	s.serial++
	if s.timer != nil {
		if err := s.timer.Close(); err != nil {
			return err
		}
		s.timer = nil
	}
	s.callback = nil
	return nil
}
