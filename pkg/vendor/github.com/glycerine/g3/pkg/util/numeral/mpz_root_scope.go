package numeral

// Source: src/util/mpz.cpp:1942-2021; src/util/mpz.h:836-837.

import (
	"errors"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrRootPending = errors.New("mpnum: retained integer root RAW invocation")

// Four actual RAW numeral fields are charged in the enclosing Integer header.
// Body errors do not execute the source end-only ascending Del sequence.
type integerRootScope struct {
	powerInvocation                      integerPowerInvocationReceipt
	Values                               [4]Int
	pending                              SourceCellBacking
	raw                                  [3]umemory.Block
	generation                           uint64
	active, EndEntered, stopped          bool
	SourceIndex, cleanupIndex            uint8
	bodyCause, sourceCause, cleanupCause error
}

func (m *IntegerManager) finishRoot() error {
	s := &m.RootRaw
	if s.stopped {
		return s.sourceCause
	}
	if !s.active || !s.EndEntered {
		return nil
	}
	for s.SourceIndex < 4 {
		if _, e := SourceNumeralDel(m, &s.Values[s.SourceIndex]); e != nil {
			s.stopped = true
			s.sourceCause = e
			return e
		}
		s.SourceIndex++
	}
	s.active = false
	return nil
}

// CleanupRoot is an explicit owning abort, never a source end-label
// execution. It resumes only these actual four raw fields in ascending order.
func (m *IntegerManager) CleanupRoot() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.RootRaw
	if e := m.CleanupPowerInvocation(&s.powerInvocation, false); e != nil {
		return e
	}
	if !s.active {
		return nil
	}
	for s.cleanupIndex < 4 {
		p := &s.Values[s.cleanupIndex]
		var e error
		if m.SourceFinalized || m.ConstructorUnwound {
			if p.Cell != 0 && !m.IsFailureRef(p.Cell) {
				e = m.freeAfterSourceFinalize(p.Cell)
			}
			if e == nil {
				p.Cell = 0
			}
		} else {
			e = m.Del(p)
		}
		if e != nil {
			s.cleanupCause = e
			return e
		}
		s.cleanupIndex++
	}
	if s.pending.Object != (umemory.ObjectRef{}) {
		if m.PendingCell == s.pending {
			if e := m.cleanupPendingAllocation(); e != nil {
				s.pending = m.PendingCell
				s.cleanupCause = e
				return e
			}
		}
		s.pending = SourceCellBacking{}
	}
	if s.raw != ([3]umemory.Block{}) {
		now, e := m.Allocator.PendingRoots()
		if e != nil {
			return e
		}
		if now == s.raw {
			e = m.Allocator.CleanupPending()
			after, x := m.Allocator.PendingRoots()
			s.raw = after
			if e != nil {
				s.cleanupCause = e
				return errors.Join(e, x)
			}
			if x != nil {
				return x
			}
		}
		s.raw = [3]umemory.Block{}
	}
	if e := m.CleanupPowerInvocation(&s.powerInvocation, true); e != nil {
		return e
	}
	s.active = false
	s.cleanupCause = nil
	return nil
}

// Only actual internal RAW-local operations acquire pending physical roots.
// Final SET(root,lo/hi) is caller-owned and deliberately bypasses this adapter.
func (m *IntegerManager) rootRawCall(op uint8, a, b, out *Int) (cause error) {
	before := m.PendingCell.Object
	raw, e := m.Allocator.PendingRoots()
	if e != nil {
		return e
	}
	switch op {
	case 0:
		cause = m.powerWithReceipt(a, uint32(b.Val), out, &m.RootRaw.powerInvocation)
	case 1:
		cause = m.Add(a, b, out)
	case 2:
		cause = m.machineDiv2kCore(out, uint32(b.Val))
	case 3:
		cause = m.Div(a, b, out)
	}
	s := &m.RootRaw
	if before == (umemory.ObjectRef{}) && m.PendingCell.Object != (umemory.ObjectRef{}) {
		if !m.matchesPowerInvocation(&s.powerInvocation) || m.PowerRaw.pending != m.PendingCell {
			s.pending = m.PendingCell
		}
	}
	after, e := m.Allocator.PendingRoots()
	if raw == ([3]umemory.Block{}) && after != ([3]umemory.Block{}) {
		if !m.matchesPowerInvocation(&s.powerInvocation) || m.PowerRaw.raw != after {
			s.raw = after
		}
	}
	if e != nil {
		return errors.Join(cause, e)
	}
	return cause
}

type integerRootInvocationReceipt struct {
	manager    *IntegerManager
	scope      *integerRootScope
	invocation uint64
}

func (m *IntegerManager) matchesRootInvocation(r *integerRootInvocationReceipt) bool {
	return r != nil && r.manager == m && r.scope == &m.RootRaw && r.invocation != 0 && r.scope.generation == r.invocation && r.scope.active
}
func (m *IntegerManager) RootInvocationSourceCause(r *integerRootInvocationReceipt) error {
	if m.matchesRootInvocation(r) {
		if e := m.PowerInvocationSourceCause(&r.scope.powerInvocation); e != nil {
			return e
		}
	}
	if m.matchesRootInvocation(r) && r.scope.stopped {
		return r.scope.sourceCause
	}
	return nil
}
func (m *IntegerManager) CleanupRootInvocation(r *integerRootInvocationReceipt, abort bool) error {
	if !m.matchesRootInvocation(r) {
		return nil
	}
	if e := m.CleanupPowerInvocation(&r.scope.powerInvocation, false); e != nil {
		return e
	}
	if !abort && !r.scope.EndEntered {
		return nil
	}
	return m.CleanupRoot()
}
func (m *IntegerManager) beginRootRawScope() error { return m.beginRootRawScopeWithReceipt(nil) }
func (m *IntegerManager) beginRootRawScopeWithReceipt(receipt *integerRootInvocationReceipt) error {
	s := &m.RootRaw
	if s.active {
		return ErrRootPending
	}
	if m.rootGeneration == ^uint64(0) {
		return ErrRootPending
	}
	m.rootGeneration++
	*s = integerRootScope{active: true, generation: m.rootGeneration}
	if receipt != nil {
		*receipt = integerRootInvocationReceipt{m, s, s.generation}
	}
	return nil
}
func (m *IntegerManager) endRootRawScope() error {
	m.RootRaw.EndEntered = true
	return m.finishRoot()
}

// Caller capture changes only ownership of unpublished allocation prefixes.
// The actual Root/Power raw numerals and original algorithms remain unchanged.
func (m *IntegerManager) rootRawCallWithContext(op uint8, a, b, out *Int, context integerAllocationInvocationContext) error {
	if context == (integerAllocationInvocationContext{}) {
		return m.rootRawCall(op, a, b, out)
	}
	switch op {
	case 0:
		return m.powerWithAllocationContext(a, uint32(b.Val), out, &m.RootRaw.powerInvocation, context)
	case 1:
		return m.addSubWithAllocationContext(a, b, out, false, context)
	case 2:
		return m.machineDiv2kCore(out, uint32(b.Val))
	case 3:
		return m.machineDivWithAllocationContext(a, b, out, context)
	}
	return ErrIntegerAssertion
}
