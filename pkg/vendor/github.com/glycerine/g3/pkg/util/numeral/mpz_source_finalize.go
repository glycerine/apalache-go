package numeral

// Source: src/util/mpz.cpp:142–161 and reverse member destruction in src/util/mpz.h and src/util/mpn.h.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// SourceFinalize runs only the source destructor, preserving caller-owned
// direct allocations. CloseChecked is the separate owning Go cleanup adapter.
// The charged scalar cursor records completed actions across cleanup refusal;
// it does not claim C++ recovery from an interrupted destructor.
func (m *IntegerManager) SourceFinalize() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.rawInvocationExecuting() || m.CopyAllocation.Executing || m.NormalizationExecuting || m.numericCallerExecuting() {
		return ErrNumericPending
	}
	if m.closed || m.SourceFinalized {
		return nil
	}
	if m.LCMPending {
		return ErrIntegerLCMPending
	}
	if m.Smt2Display.active {
		return ErrSMT2DisplayPending
	}
	// Constructor failure is not a completed C++ object destructor invocation.
	if m.originalCause != nil {
		return arena.ErrClosed
	}
	if e := m.stringSetSourceReady(); e != nil {
		return e
	}
	if e := m.finishSMT2StreamSource(); e != nil {
		return e
	}
	if e := m.finishMyMPZSqrtSource(); e != nil {
		return e
	}
	m.SourceFinalizing = true
	fail := func(e error) error { m.CleanupCauseField = e; return e }
	for m.SourceFinalizeStage < 8 {
		switch m.SourceFinalizeStage {
		case 0, 1:
			value := &m.Two64
			if m.SourceFinalizeStage == 1 {
				value = &m.IntMin
			}
			if value.Cell == m.FailedNullRef {
				value.Cell = 0
			}
			if value.Cell != 0 {
				if !ValidNumericRef(m.Cells, value.Cell) {
					m.sourceGuard = true
					return fail(arena.ErrInvalidSlot)
				}
				if e := m.free(value.Cell); e != nil {
					return fail(e)
				}
				value.Cell = 0 // del preserves m_val.
			}
		case 2, 3, 4, 5:
			index := int(m.SourceFinalizeStage-2) / 2
			ref := &m.Tmp[index]
			if m.SourceFinalizeStage%2 == 1 {
				ref = &m.Arg[index]
			}
			// Unlike del(numeral), source deallocate(ptr) reads the capacity header.
			// A missing scratch pointer is guarded at this actual destructor access.
			if *ref == 0 || !ValidNumericRef(m.Cells, *ref) {
				m.sourceGuard = true
				return fail(arena.ErrInvalidSlot)
			}
			if e := m.free(*ref); e != nil {
				return fail(e)
			}
			*ref = 0
		case 6:
			if m.Natural != nil {
				if e := m.Natural.SourceFinalize(); e != nil {
					return fail(e)
				}
			}
		case 7:
			e := m.Allocator.Close()
			// Chunk frees may complete before a later TLS sync/diagnostic refusal.
			// Mark only physically consumed pooled roots, preserving surviving chunks.
			if mark := m.markSourceFreedPools(); mark != nil {
				return fail(mark)
			}
			if e != nil {
				return fail(e)
			}
		}
		m.SourceFinalizeStage++
	}
	m.SourceFinalized = true
	m.CleanupCauseField = nil
	return nil
}

func (m *IntegerManager) markSourceFreedPools() error {
	if parent := m.rationalServiceParent; parent != nil {
		if parent.self != parent {
			return ErrNumericOwner
		}
		for c := parent.lexicalInvocations; c != nil; c = c.next {
			if !c.valid() {
				return ErrNumericOwner
			}
			if c.integer != m {
				continue
			}
			if e := m.markSourceConsumedDetachedCell(&c.numeric.Cell, &c.cellRetired); e != nil {
				return e
			}
		}
	}

	if parent := m.MpfParent; parent != nil && parent.self == parent && parent.integer == m {
		for _, s := range []*ArbitraryFloatRoundScope{&parent.RoundScope, &parent.RoundSqrtScope} {
			if s.Manager == parent && s.Invocation != 0 {
				if e := m.markSourceConsumedDetachedCell(&s.Numeric.Cell, &s.cellRetired); e != nil {
					return e
				}
			}
		}
		for h := parent.RationalSetHistories; h != nil; h = h.Next {
			if h.Self != h || h.Manager != parent || h.Invocation == 0 {
				return umemory.ErrRuntimeOwner
			}
			if e := m.markSourceConsumedDetachedCell(&h.Numeric.Cell, &h.cellRetired); e != nil {
				return e
			}
		}
		for h := parent.FormattedSetHistories; h != nil; h = h.Next {
			if h.Self != h || h.Manager != parent || h.Issued.Manager != parent || h.Issued.Frame != h || h.Issued.Invocation != h.Invocation || h.Invocation == 0 {
				return umemory.ErrRuntimeOwner
			}
			if e := m.markSourceConsumedDetachedCell(&h.Numeric.Cell, &h.cellRetired); e != nil {
				return e
			}
		}
		for q := parent.InputCalls; q != nil; q = q.Next {
			if q.Self != q || q.Manager != parent || q.Issued.Manager != parent || q.Issued.Frame != q || q.Issued.Invocation == 0 {
				return umemory.ErrRuntimeOwner
			}
			if e := m.markSourceConsumedDetachedCell(&q.Numeric.Cell, &q.cellRetired); e != nil {
				return e
			}
		}
	}

	for s := m.conversionDetachedOutputs; s != nil; s = s.outputDetachedNext {
		if s.manager == nil || !s.manager.validConversionScope(s) || s.outputManager.Z != m || s.outputDetachedInvocation == 0 || s.outputDetachedInvocation != s.invocation {
			return umemory.ErrRuntimeOwner
		}
		if e := m.markSourceConsumedDetachedCell(&s.outputAllocation.numeric.Cell, &s.outputAllocation.cellRetired); e != nil {
			return e
		}
	}
	if parent := m.MpfParent; parent != nil {
		s := &parent.rationalConversion
		if parent.validConversionScope(s) {
			if e := m.markSourceConsumedDetachedCell(&s.numeric.Cell, &s.cellRetired); e != nil {
				return e
			}
		}
	}

	if parent := m.MpfParent; parent != nil {
		for s := parent.FormatScopes; s != nil; s = s.Next {
			if !parent.MatchesFormatFrame(s) {
				return ErrNumericOwner
			}
			if e := m.markSourceConsumedDetachedCell(&s.Numeric.Cell, &s.cellRetired); e != nil {
				return e
			}
			if e := m.markSourceConsumedDetachedCell(&s.Decimal.Numeric.Cell, &s.Decimal.cellRetired); e != nil {
				return e
			}
		}
	}

	if parent := m.MpfParent; parent != nil {
		for q := parent.ValueCalls; q != nil; q = q.Next {
			if q.Retired || !m.MatchesCopyAllocationInvocation(&q.Issued.Child) {
				continue
			}
			child := &q.ChildFrame
			if e := m.markSourceConsumedDetachedCell(&child.Roots.Cell, &child.cellRetired); e != nil {
				return e
			}
		}
	}
	if parent := m.MpfParent; parent != nil {
		for w := parent.PowerWrappers; w != nil; w = w.Next {
			if !w.Valid() || w.manager.integer != m {
				return ErrNumericOwner
			}
			if e := m.markSourceConsumedDetachedCell(&w.Roots.Cell, &w.cellRetired); e != nil {
				return e
			}
		}
	}
	for n := uint64(1); n <= uint64(m.Cells.Stats().Slots); n++ {
		slot := arena.Slot(n)
		c, e := m.Cells.Read(slot)
		if e != nil || c.Released || c.Backing.Offset == 0 {
			continue
		}
		_, e = m.Memory.ReadUint32(c.Backing.Raw, c.Backing.Offset+4)
		if errors.Is(e, umemory.ErrRuntimeOwner) {
			c.Released = true
			if e := m.Cells.Write(slot, c); e != nil {
				return e
			}
		} else if e != nil {
			return e
		}
	}
	return nil
}

// After source allocator destruction its locator directory no longer exists.
// Each charged descriptor retains the real raw allocation token; owning cleanup
// frees direct blocks through Runtime, without an invented allocator event.
func (m *IntegerManager) freeAfterSourceFinalize(ref NumericCellRef) error {
	if m.IsFailureRef(ref) {
		return nil
	} // actual owning metadata, never a raw source free

	c := m.Cell(ref)
	if !c.Released {
		if c.Backing.Offset != 0 {
			return arena.ErrInvalidSlot
		}
		raw := c.Backing.Raw
		e := m.Memory.Deallocate(&raw)
		if raw == (umemory.Block{}) {
			c.Released = true
			m.Store(ref, c)
		}
		if e != nil {
			c.CleanupPending = true
			m.Store(ref, c)
			return e
		}
	}
	return m.Cells.Recycle(NumericSlot(ref))
}

// markSourceConsumedDetachedCell checks only a previously acquired exact cell
// token at the reached allocator destructor frontier. Direct blocks survive.
func (m *IntegerManager) markSourceConsumedDetachedCell(c *SourceCellBacking, retired *bool) error {
	if c.Object == (umemory.ObjectRef{}) || c.Offset == 0 {
		return nil
	}
	_, e := m.Memory.ReadUint32(c.Raw, c.Offset+4)
	if errors.Is(e, umemory.ErrRuntimeOwner) {
		*c = SourceCellBacking{}
		*retired = true
		return nil
	}
	return e
}
