package control

// Source: src/util/scoped_ctrl_c.h:24-41; src/util/scoped_ctrl_c.cpp:24-66.

import (
	"os"
	"os/signal"
	"sync"
)

var scopedControlCState struct {
	sync.Mutex
	top     *ScopedCtrlC
	signals chan os.Signal
	stop    chan struct{}
}

// ScopedCtrlC installs a process interrupt handler for its lifetime. Enabled
// scopes must be closed in reverse construction order. When once is true, the
// first interrupt invokes Handler and the second is forwarded to the handler
// that would otherwise receive os.Interrupt.
type ScopedCtrlC struct {
	self    *ScopedCtrlC
	handler EventHandler
	old     *ScopedCtrlC
	once    bool
	first   bool
	enabled bool
	closed  bool
}

// NewScopedCtrlC constructs and optionally installs a scoped interrupt handler.
func NewScopedCtrlC(handler EventHandler, once, enabled bool) (*ScopedCtrlC, error) {
	if handler == nil {
		return nil, ErrPrecondition
	}
	if function, ok := handler.(EventHandlerFunc); ok && function == nil {
		return nil, ErrPrecondition
	}
	scope := &ScopedCtrlC{handler: handler, once: once, first: true, enabled: enabled}
	scope.self = scope

	if !enabled {
		return scope, nil
	}
	scopedControlCState.Lock()
	scope.old = scopedControlCState.top
	scopedControlCState.top = scope
	if scopedControlCState.signals == nil {
		startScopedControlCLocked()
	}
	scopedControlCState.Unlock()
	return scope, nil
}

func startScopedControlCLocked() {
	signals := make(chan os.Signal, 1)
	stop := make(chan struct{})
	scopedControlCState.signals = signals
	scopedControlCState.stop = stop
	signal.Notify(signals, os.Interrupt)
	go func() {
		for {
			select {
			case <-signals:
				if dispatchScopedControlC() {
					forwardScopedControlC(signals, stop)
					return
				}
			case <-stop:
				return
			}
		}
	}()
}

// dispatchScopedControlC returns true when the previous process handler should
// receive the interrupt. It is separate from OS delivery so state transitions
// can be tested without sending a terminating signal to the test process.
func dispatchScopedControlC() bool {
	scopedControlCState.Lock()
	scope := scopedControlCState.top
	if scope == nil || scope.closed {
		scopedControlCState.Unlock()
		return true
	}
	if !scope.first {
		scopedControlCState.Unlock()
		return true
	}
	handler := scope.handler
	scopedControlCState.Unlock()

	handler.HandleEvent(EventCallerControlC)

	scopedControlCState.Lock()
	if !scope.closed && scope.once {
		scope.first = false
	}
	scopedControlCState.Unlock()
	return false
}

func forwardScopedControlC(signals chan os.Signal, stop chan struct{}) {
	scopedControlCState.Lock()
	if scopedControlCState.signals != signals || scopedControlCState.stop != stop {
		scopedControlCState.Unlock()
		return
	}
	signal.Stop(signals)
	scopedControlCState.signals = nil
	scopedControlCState.stop = nil
	scopedControlCState.Unlock()

	process, err := os.FindProcess(os.Getpid())
	if err == nil {
		_ = process.Signal(os.Interrupt)
	}
}

// Reset allows a once scope to handle the next interrupt as its first one.
func (scope *ScopedCtrlC) Reset() error {
	if scope == nil || scope.self != scope {
		return ErrOwner
	}
	scopedControlCState.Lock()
	defer scopedControlCState.Unlock()
	if scope.closed {
		return ErrOwner
	}
	scope.first = true
	return nil
}

// Close restores the previously active scoped handler.
func (scope *ScopedCtrlC) Close() error {
	if scope == nil || scope.self != scope {
		return ErrOwner
	}
	scopedControlCState.Lock()
	defer scopedControlCState.Unlock()
	if scope.closed {
		return nil
	}
	if scope.enabled && scopedControlCState.top != scope {
		return ErrPrecondition
	}
	if scope.enabled {
		scopedControlCState.top = scope.old
		if scopedControlCState.top == nil && scopedControlCState.signals != nil {
			signal.Stop(scopedControlCState.signals)
			close(scopedControlCState.stop)
			scopedControlCState.signals = nil
			scopedControlCState.stop = nil
		}
	}
	scope.closed = true
	scope.handler = nil
	scope.old = nil
	return nil
}

func (scope *ScopedCtrlC) Closed() bool {
	if scope == nil || scope.self != scope {
		return true
	}
	scopedControlCState.Lock()
	defer scopedControlCState.Unlock()
	return scope.closed
}
