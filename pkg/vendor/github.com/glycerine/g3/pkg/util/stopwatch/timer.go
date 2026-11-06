package stopwatch

// Source: src/util/timer.h:27-36; src/util/timer.cpp:24-39.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var ErrTimerOwner = errors.New("g3: invalid timer owner")
var ErrTimerPending = errors.New("g3: timer operation in progress")
var ErrTimerClosed = errors.New("g3: timer closed")

// Timer measures accumulated process CPU time. A nil clock selects the
// platform clock. It owns its stopwatch directly and must not be copied.
type Timer struct {
	self           *Timer
	watch          Stopwatch
	memoryTrace    *diagnostic.TraceTags
	cause          error
	active, closed bool
}

func NewTimer(clock func() (uint64, error)) (*Timer, error) {
	return NewTimerWithMemoryTrace(clock, nil)
}

func NewTimerWithMemoryTrace(clock func() (uint64, error), trace *diagnostic.TraceTags) (*Timer, error) {
	if trace == nil {
		trace = diagnostic.BorrowProcessTraceTags()
	}
	p := &Timer{watch: New(clock), memoryTrace: trace, active: true}
	p.self = p
	p.traceWatchAllocated()
	p.cause = p.watch.Start()
	p.active = false
	return p, p.cause
}

func (p *Timer) open() error {
	if p == nil || p.self != p {
		return ErrTimerOwner
	}
	if p.closed {
		return ErrTimerClosed
	}
	if p.active {
		return ErrTimerPending
	}
	return p.cause
}

func (p *Timer) Start() error {
	if e := p.open(); e != nil {
		return e
	}
	p.active = true
	defer func() { p.active = false }()
	return p.watch.Start()
}

// GetSeconds stops then restarts a running watch, preserving accumulated time
// and the reached stop state if the restart clock read fails.
func (p *Timer) GetSeconds() (float64, error) {
	if e := p.open(); e != nil {
		return 0, e
	}
	p.active = true
	defer func() { p.active = false }()
	return p.watch.Seconds()
}

func (p *Timer) Timeout(seconds uint32) (bool, error) {
	if seconds == 0 || seconds == ^uint32(0) {
		return false, nil
	}
	elapsed, e := p.GetSeconds()
	return elapsed > float64(seconds), e
}

func (p *Timer) MSTimeout(milliseconds uint32) (bool, error) {
	if milliseconds == 0 || milliseconds == ^uint32(0) {
		return false, nil
	}
	elapsed, e := p.GetSeconds()
	return elapsed*1000 > float64(milliseconds), e
}

func (p *Timer) Close() error {
	if p == nil || p.self != p {
		return ErrTimerOwner
	}
	if p.active {
		return ErrTimerPending
	}
	if p.closed {
		return nil
	}
	p.closed = true
	p.traceWatchDeallocated()
	return nil
}
