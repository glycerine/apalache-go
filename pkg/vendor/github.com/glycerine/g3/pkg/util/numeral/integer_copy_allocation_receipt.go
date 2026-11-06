package numeral

// Source: src/util/mpz.cpp:big_set; src/util/mpz.h:allocate.

import (
	"errors"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

type integerCopyAllocationScope struct {
	invocation        uint64
	active, Executing bool
	Roots             NumericAllocationRoots
	mpfCaller         *arbitraryFloatValueCopyCall
	Transferred       umemory.SmallAllocatorPendingAllocation
	originalCell      SourceCellBacking
	cellRetired       bool
}
type integerCopyAllocationReceipt struct {
	manager    *IntegerManager
	Frame      *integerCopyAllocationScope
	invocation uint64
}

func (m *IntegerManager) validCopyAllocationScope(s *integerCopyAllocationScope) bool {
	if m == nil || m.self != m || s == nil {
		return false
	}
	if s == &m.CopyAllocation {
		return true
	}
	q := s.mpfCaller
	if q == nil || q.self != q || s != &q.ChildFrame || q.header == 0 || q.Retired {
		return false
	}
	owner := q.manager
	if owner == nil || owner.self != owner || owner.closed || owner.integer != m || q.Issued.manager != owner || q.Issued.frame != q || q.Issued.invocation == 0 || q.Issued.Child.manager != m || q.Issued.Child.Frame != s || q.Issued.Child.invocation != s.invocation {
		return false
	}
	for old := owner.ValueCalls; old != nil; old = old.Next {
		if old == q {
			return true
		}
	}
	return false
}

func (m *IntegerManager) executingCopyAllocationScope(s *integerCopyAllocationScope) bool {
	if !m.validCopyAllocationScope(s) || !s.active || !s.Executing {
		return false
	}
	if s.mpfCaller == nil {
		return true
	}
	q := s.mpfCaller
	c := &q.manager.valueBinding
	return q.executing && c.record == q && c.invocation == s.invocation && m.validateFloatCopyBinding(c) == nil
}

func (m *IntegerManager) MatchesCopyAllocationInvocation(r *integerCopyAllocationReceipt) bool {
	return r != nil && r.manager == m && m.validCopyAllocationScope(r.Frame) && r.invocation != 0 && r.Frame.invocation == r.invocation && r.Frame.active
}
func (m *IntegerManager) CleanupCopyAllocationInvocation(r *integerCopyAllocationReceipt) error {
	if !m.MatchesCopyAllocationInvocation(r) {
		return nil
	}
	s := r.Frame
	if s.Executing || (s.mpfCaller != nil && m.Memory.FailureTransportActive()) {
		return ErrNumericPending
	}
	if s.mpfCaller != nil {
		if s.Roots.Cell.Object != (umemory.ObjectRef{}) {
			e := m.cleanupDetachedCopyCell(&s.Roots.Cell)
			if s.Roots.Cell.Object == (umemory.ObjectRef{}) {
				s.cellRetired = true
			}
			if e != nil {
				return e
			}
		}
		if s.Transferred.Block() != (umemory.Block{}) {
			e := m.Allocator.CleanupTransferredAllocation(&s.Transferred)
			s.Roots.Allocator[0] = s.Transferred.Block()
			if e != nil {
				return e
			}
		}
	} else if e := s.Roots.Cleanup(m); e != nil {
		return e
	}
	*s = integerCopyAllocationScope{invocation: s.invocation, mpfCaller: s.mpfCaller, originalCell: s.originalCell, cellRetired: s.cellRetired || s.originalCell.Object != (umemory.ObjectRef{})}
	return nil
}

// A descriptor-preparation failure retains this exact backing outside cells.
// Source chunk destruction may consume pooled backing before its own suffix
// stops. Direct backing survives allocator destruction and belongs to caller.
func (m *IntegerManager) cleanupDetachedCopyCell(c *SourceCellBacking) error {
	if c.Offset != 0 {
		if _, e := m.Memory.Size(c.Raw); errors.Is(e, umemory.ErrRuntimeOwner) {
			*c = SourceCellBacking{}
			return nil
		} else if e != nil {
			return e
		}
	}
	ended, e := m.Allocator.SourceObjectOwnerEnded()
	if e != nil {
		return e
	}
	if ended && c.Offset == 0 {
		raw := c.Raw
		e = m.Memory.Deallocate(&raw)
		if raw == (umemory.Block{}) {
			*c = SourceCellBacking{}
		}
		return e
	}
	if c.Offset == 0 {
		raw := c.Raw
		handled, e := m.Allocator.CleanupSourceRetainedDirect(c.Object, &raw)
		if handled {
			if raw == (umemory.Block{}) {
				*c = SourceCellBacking{}
			}
			return e
		}
	}
	progress, e := c.deallocate(m.Allocator)
	if progress.PhysicalReleased || progress.PooledLinked {
		*c = SourceCellBacking{}
	}
	return e
}
