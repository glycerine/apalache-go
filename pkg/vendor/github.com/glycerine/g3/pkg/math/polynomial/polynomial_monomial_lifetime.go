package polynomial

// Source: src/math/polynomial/polynomial.cpp; src/math/polynomial/polynomial.h.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// Constructor unwind invokes only destructors of completed members. A failed
// raw allocation or body-created pointer remains an explicitly retained owner.
func (m *MonomialManager) constructorUnwind() error {
	for i := range m.temps {
		m.temps[i] = monomialTemp{}
	}
	if m.idsInitialized {
		m.ids.Cleanup(0)
	}
	m.table.cleanup()
	if m.creation.stage != 0 {
		m.creation.tableRemoved = true
	}
	return nil
}
func (m *MonomialManager) externalEntries() bool {
	allowed := uint32(0)
	if m.unit != (Monomial{}) {
		allowed = 1
	}
	return m.publishedObjects > allowed
}

// SourceFinalize executes the completed source destructor body, then reverse
// member destruction. It never manufactures destruction of caller monomials.
func (m *MonomialManager) SourceFinalize() error {
	if m != nil && m.self == m && (m.assertionRunning || m.queryTraceRunning || m.managerTraceRunning || (m.substitutionTraceCallback() || (m.exactDivTraceCallback() || m.lexTraceCallback()))) {
		return ErrMonomialPending
	}
	if m == nil || m.self != m {
		return ErrMonomialOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	if !m.completed || m.originalCause != nil {
		return arena.ErrClosed
	}
	if m.closing {
		return arena.ErrClosed
	}
	m.sourceFinalizing = true
	if m.sourceStage == 0 {
		if m.unitDropCause != nil {
			return m.unitDropCause
		}
		out := monomialDecrementOutcome{}
		e := m.decRefCoreOutcome(m.unit, &out)
		if out.written && out.last {
			m.unitDropConsumed = true
		}
		if e != nil {
			m.unitDropCause = e
			return e
		}
		m.unit = Monomial{}
		m.sourceStage++
	}
	if m.sourceStage == 1 {
		if e := m.traceLeakedMonomials(); e != nil {
			return e
		}
		if e := m.sourceScalarAssert(801, "m_monomials.empty()", m.table.entries.Len() == 0); e != nil {
			return e
		}

		m.sourceStage++
	}
	if m.sourceStage == 2 {
		if m.allocatorOwned {
			if e := m.allocator.Close(); e != nil {
				return e
			}
			if m.allocatorMemoryTraceLive {
				m.traceOwnedAllocatorDeallocated()
				m.allocatorMemoryTraceLive = false
			}
		}
		m.sourceStage++
	}
	if m.sourceStage == 3 {
		if e := m.powerVectorSourceDestroy(); e != nil {
			return e
		}
		m.sourceStage++
	}
	for m.sourceStage >= 4 && m.sourceStage < 8 {
		m.temps[7-m.sourceStage] = monomialTemp{}
		m.sourceStage++
	}

	if m.sourceStage == 8 {
		if m.idsInitialized {
			m.ids.Cleanup(0)
		}
		m.sourceStage++
	}
	if m.sourceStage == 9 {
		m.table.cleanup()
		if m.creation.stage != 0 {
			m.creation.tableRemoved = true
		}
		m.sourceStage++
	}
	return nil
}

// CleanupValue is an explicit owning abort of a caller-supplied value after a
// partial source mutation. It removes its actual table edge; no locator sweep.
func (m *MonomialManager) CleanupValue(a *Monomial) error {
	if m != nil && m.self == m && (m.assertionRunning || m.queryTraceRunning || m.managerTraceRunning || (m.substitutionTraceCallback() || (m.exactDivTraceCallback() || m.lexTraceCallback()))) {
		return ErrMonomialPending
	}
	if m == nil || m.self != m || m.closed {
		return ErrMonomialOwner
	}
	if a == nil {
		return arena.ErrBounds
	}
	if *a == (Monomial{}) {
		return nil
	}
	l, e := m.location(*a)
	if e != nil {
		return e
	}
	m.table.removeToken(monomialKey{monomialObjectKey, a.slot})
	if l.raw != (umemory.ObjectRef{}) && m.sourceStage >= 3 && m.allocatorOwned {
		if l.sourceOffset == 0 {
			e = m.memory.Deallocate(&l.sourceRaw)
			if l.sourceRaw == (umemory.Block{}) {
				l.raw = umemory.ObjectRef{}
			}
			if e != nil {
				return e
			}
		} else {
			if _, e = m.memory.Read(l.sourceRaw, 0); e == nil {
				return ErrMonomialPending
			}
		}
		l.raw = umemory.ObjectRef{}
	}
	if l.raw != (umemory.ObjectRef{}) {
		p, e := m.allocator.DeallocateTracked(uint64(l.requested), l.raw)
		if p.PhysicalReleased || p.PooledLinked {
			l.acknowledgeToPolynomialCreated(*a)
			l.raw = umemory.ObjectRef{}
		}
		if e != nil {
			return e
		}
	}
	if l.published {
		m.publishedObjects--
		l.published = false
	}
	if d := l.conversionCohort; d != nil && uint64(l.conversionSlot) < uint64(len(d.orphans)) && d.orphans[l.conversionSlot] == *a {
		d.orphans[l.conversionSlot] = Monomial{}
	}
	l.conversionCohort = nil
	l.conversionSlot = 0
	if d := l.squareRootCohort; d != nil && uint64(l.squareRootSlot) < uint64(len(d.orphans)) && d.orphans[l.squareRootSlot] == *a {
		d.orphans[l.squareRootSlot] = Monomial{}
		d.orphanInvocations[l.squareRootSlot] = 0
	}
	l.squareRootCohort = nil
	l.squareRootSlot = 0
	if d := l.derivativeCohort; d != nil && uint64(l.derivativeSlot) < uint64(len(d.orphans)) && d.orphans[l.derivativeSlot] == *a {
		d.orphans[l.derivativeSlot] = Monomial{}
		d.orphanInvocations[l.derivativeSlot] = 0
	}
	if d := l.compositionCohort; d != nil && uint64(l.compositionSlot) < uint64(len(d.orphans)) && d.orphans[l.compositionSlot] == *a {
		d.orphans[l.compositionSlot] = Monomial{}
	}
	l.compositionCohort = nil
	l.compositionSlot = 0
	l.derivativeCohort = nil
	l.derivativeSlot = 0
	l.acknowledgeArithmeticProduct(*a)
	l.acknowledgePseudoCreated(*a)
	l.acknowledgeToPolynomialCreated(*a)
	l.live = false
	*a = Monomial{}
	return nil
}
func (m *MonomialManager) CloseChecked() error {
	if m != nil && m.self == m && (m.assertionRunning || m.queryTraceRunning || m.managerTraceRunning || (m.substitutionTraceCallback() || (m.exactDivTraceCallback() || m.lexTraceCallback()))) {
		return ErrMonomialPending
	}
	if m == nil {
		return nil
	}
	if m.self != m {
		return ErrMonomialOwner
	}
	if m.closed {
		return nil
	}
	if !m.closing && m.unit != (Monomial{}) {
		l, e := m.location(m.unit)
		if e != nil {
			return e
		}
		// A real stopped deletion has consumed its logical references. Its
		// retained progress is authoritative; the old raw may already be free.
		if l.stage == 0 {
			h, e := m.headerCore(m.unit)
			if e != nil {
				return e
			}
			if h.RefCount > 1 {
				return ErrMonomialLive
			}
		}
	}
	if !m.closing && m.externalEntries() {
		return ErrMonomialLive
	}
	m.closing = true
	if e := m.CleanupRename(); e != nil {
		return e
	}
	if e := m.CleanupCreation(); e != nil {
		m.cleanupCause = e
		return e
	}
	if m.unit != (Monomial{}) {
		l, e := m.location(m.unit)
		if e != nil {
			return e
		}
		if l.stage != 0 || m.unitDropConsumed || m.originalCause != nil {
			if e = m.CleanupValue(&m.unit); e != nil {
				return e
			}
		} else {
			if e := m.decRefCore(m.unit); e != nil {
				m.cleanupCause = e
				return e
			}
			m.unit = Monomial{}
		}
	}
	if e := m.powerVectorCleanup(); e != nil {
		return e
	}
	for i := 3; i >= 0; i-- {
		if e := m.tempCleanup(&m.temps[i]); e != nil {
			return e
		}
	}
	if m.idsInitialized {
		m.ids.Cleanup(0)
		m.idsInitialized = false
	}
	m.table.cleanup()
	if m.allocatorOwned && m.allocator != nil {
		if e := m.allocator.CleanupPending(); e != nil {
			return e
		}
		if e := m.allocator.Close(); e != nil {
			return e
		}
		if m.allocatorMemoryTraceLive && m.completed && !m.sourceFinalizing {
			m.traceOwnedAllocatorDeallocated()
			m.allocatorMemoryTraceLive = false
		}
	}
	if m.memoryOwned {
		if e := m.memory.Close(); e != nil {
			return e
		}
	}
	m.locations = nil
	m.config.Budget.Release(m.header)
	if m.memoryTraceLive && m.completed && (!m.sourceFinalizing || m.sourceStage >= 10) {
		m.tracePolynomialImpDeallocated()
		m.memoryTraceLive = false
	}
	m.header = 0
	m.closed = true
	m.cleanupCause = nil
	return nil
}
func (m *MonomialManager) Close() {
	if e := m.CloseChecked(); e != nil {
		m.cleanupCause = e
		panic(m)
	}
}
func (m *MonomialManager) IncManagerRef() error {
	if e := m.check(); e != nil {
		return e
	}
	m.managerRefs++
	return nil
}
func (m *MonomialManager) DecManagerRef() error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.sourceScalarAssert(811, "m_ref_count > 0", m.managerRefs > 0); e != nil {
		return e
	}
	m.managerRefs--
	if m.managerRefs == 0 {
		if e := m.SourceFinalize(); e != nil {
			return e
		}
		return m.CloseChecked()
	}
	return nil
}
