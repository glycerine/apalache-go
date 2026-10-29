package numeral

// Source: src/util/mpz.cpp:1453-1507; src/util/mpz.h:778.

import (
	"errors"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrPowerPending = errors.New("mpnum: retained integer power RAW invocation")

// One actual RAW numeral field are charged in the enclosing Integer header.
// Body errors do not execute the source end-only ascending Del sequence.
type integerPowerScope struct {
	Value                                Int
	pending                              SourceCellBacking
	raw                                  [3]umemory.Block
	generation                           uint64
	active, endEntered, stopped          bool
	sourceIndex, cleanupIndex            uint8
	bodyCause, sourceCause, cleanupCause error
}

func (m *IntegerManager) finishPower() error {
	s := &m.PowerRaw
	if s.stopped {
		return s.sourceCause
	}
	if !s.active || !s.endEntered {
		return nil
	}
	for s.sourceIndex < 1 {
		if _, e := SourceNumeralDel(m, &s.Value); e != nil {
			s.stopped = true
			s.sourceCause = e
			return e
		}
		s.sourceIndex++
	}
	s.active = false
	return nil
}

// CleanupPower is an explicit owning abort, never a source end-label
// execution. It resumes only this actual raw field in ascending order.
func (m *IntegerManager) CleanupPower() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.PowerRaw
	if !s.active {
		return nil
	}
	for s.cleanupIndex < 1 {
		p := &s.Value
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
	s.active = false
	s.cleanupCause = nil
	return nil
}

// Only internal RAW COPY and final-square acquire local pending roots.
// Output SET/MUL stay caller-owned, including a stopped late publication.
func (m *IntegerManager) powerRawCall(op uint8, a, b, out *Int) (cause error) {
	before := m.PendingCell
	raw, e := m.Allocator.PendingRoots()
	if e != nil {
		return e
	}
	switch op {
	case 0:
		cause = m.Copy(out, a)
	case 1:
		cause = m.Mul(a, b, out)
	default:
		return ErrIntegerAssertion
	}
	s := &m.PowerRaw
	if before.Object == (umemory.ObjectRef{}) && m.PendingCell.Object != (umemory.ObjectRef{}) {
		s.pending = m.PendingCell
	}
	after, e := m.Allocator.PendingRoots()
	if raw == ([3]umemory.Block{}) && after != ([3]umemory.Block{}) {
		s.raw = after
	}
	return errors.Join(cause, e)
}

// The passed concrete caller context stamps actual allocation leaves. Published
// RAW value belongs to this actual Power frame; unpublished leaves remain in
// the caller's direct roots until its later owning abort. No inventory capture.
func (m *IntegerManager) powerRawCallWithContext(op uint8, a, b, out *Int, context integerAllocationInvocationContext) error {
	if context == (integerAllocationInvocationContext{}) {
		return m.powerRawCall(op, a, b, out)
	}
	switch op {
	case 0:
		return m.copyFromOwnerWithAllocationContext(out, m, a, nil, context)
	case 1:
		return m.mulWithAllocationContext(a, b, out, context)
	}
	return ErrIntegerAssertion
}

type integerPowerInvocationReceipt struct {
	manager    *IntegerManager
	scope      *integerPowerScope
	invocation uint64
}

func (m *IntegerManager) matchesPowerInvocation(r *integerPowerInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.scope == &m.PowerRaw && r.invocation != 0 && r.scope.generation == r.invocation && r.scope.active
}
func (m *IntegerManager) PowerInvocationSourceCause(r *integerPowerInvocationReceipt) error {
	// Readonly history is exact owner/member/nonce, independent of live cleanup.
	if m != nil && m.self == m && r != nil && r.manager == m && r.scope == &m.PowerRaw && r.invocation != 0 && r.scope.generation == r.invocation && r.scope.stopped {
		return r.scope.sourceCause
	}
	return nil
}
func (m *IntegerManager) CleanupPowerInvocation(r *integerPowerInvocationReceipt, abort bool) error {
	if !m.matchesPowerInvocation(r) {
		return nil
	}
	if !abort && !r.scope.endEntered {
		return nil
	}
	return m.CleanupPower()
}
func (m *IntegerManager) beginPowerRawScope() error { return m.beginPowerRawScopeWithReceipt(nil) }
func (m *IntegerManager) beginPowerRawScopeWithReceipt(receipt *integerPowerInvocationReceipt) error {
	s := &m.PowerRaw
	if s.active {
		return ErrPowerPending
	}
	if m.powerGeneration == ^uint64(0) {
		return ErrPowerPending
	}
	m.powerGeneration++
	*s = integerPowerScope{active: true, generation: m.powerGeneration}
	if receipt != nil {
		*receipt = integerPowerInvocationReceipt{m, s, s.generation}
	}
	return nil
}
func (m *IntegerManager) endPowerRawScope() error {
	m.PowerRaw.endEntered = true
	return m.finishPower()
}
