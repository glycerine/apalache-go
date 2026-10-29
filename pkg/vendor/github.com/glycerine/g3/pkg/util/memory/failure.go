package memory

// Source: src/util/memory_manager.cpp:33-72; src/util/memory_manager.h:49-71.
// Source: src/util/error_codes.h.

import (
	"errors"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"
)

type FailureKind uint8

const (
	FailureMemout FailureKind = iota + 1
	FailureAllocExceeded
	FailureVectorOverflow
)

type Failure struct{ Kind FailureKind }

func (f Failure) Message() string {
	switch f.Kind {
	case FailureMemout:
		return "out of memory"
	case FailureAllocExceeded:
		return "number of configured allocations exceeded"
	case FailureVectorOverflow:
		return "Overflow encountered when expanding vector"
	}
	return "invalid memory failure"
}
func (f Failure) Error() string { return f.Message() }
func (f Failure) ErrorCode() uint32 {
	switch f.Kind {
	case FailureMemout:
		return 101
	case FailureAllocExceeded:
		return 113
	}
	return 0
}
func (f Failure) HasErrorCode() bool { return f.ErrorCode() != 0 }

// Transport is a single borrowed named/static function. Returned exceptions
// are cold caller-owned values; no captured environment is allocated here.
type ErrorTransport func(Failure) error

// Cold borrowed source policy; vector overflow remains a distinct transport.
type FatalPolicy interface {
	MemoryBudget() *arena.Budget
	HandleMemoryFailure(*Runtime, Failure) error
}

var ErrFailureTransport = errors.New("memory: invalid failure transport")

func (m *Runtime) emitFailure(kind FailureKind) error {
	f := Failure{Kind: kind}
	policy := m.config.FatalPolicy
	if m.config.Process != nil {
		policy = m.config.Process.fatalPolicy
	}
	if policy != nil && kind != FailureVectorOverflow {
		atomic.AddUint32(&m.failureTransportDepth, 1)
		defer atomic.AddUint32(&m.failureTransportDepth, ^uint32(0))
		e := policy.HandleMemoryFailure(m, f)
		if e == nil {
			return ErrFailureTransport
		}
		return e
	}
	if m.config.Transport != nil {
		atomic.AddUint32(&m.failureTransportDepth, 1)
		defer atomic.AddUint32(&m.failureTransportDepth, ^uint32(0))
		e := m.config.Transport(f)
		if e == nil {
			return ErrFailureTransport
		}
		return e
	}
	return f
}

// FailureTransportActive reports only the dynamic invocation of this Runtime's
// borrowed Go failure transport. It grants no block/cleanup authority. Runtime
// payload operations remain caller-confined; the atomic observation permits a
// synchronized borrowed numerical owner to avoid locking recursively in that
// invocation, without rejecting its ordinary serialized calls.
func (m *Runtime) FailureTransportActive() bool {
	return m != nil && m.self == m && atomic.LoadUint32(&m.failureTransportDepth) != 0
}
