package numeral

// Source: src/util/mpq.cpp:28-34 destructor body followed by mpz_manager base destruction.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// sourceNumeralDel checks the actual capacity access. The permanently dead
// failure token is accepted by owning cleanup, not by literal source destruction.

func sourceNumeralDel(z *IntegerManager, a *Int) (bool, error) {
	// Only the exact setter-recorded token denotes the source NULL pointer.
	// Its guarded Go metadata disappears without any capacity read or free.
	if z.FailedNullRef != 0 && a.Cell == z.FailedNullRef {
		a.Cell = 0
		return false, nil
	}
	if a.Cell != 0 {
		if !ValidNumericRef(z.Cells, a.Cell) {
			return true, arena.ErrInvalidSlot
		}
		c := z.Cell(a.Cell)
		if c.CleanupPending {
			// Persisted descriptor/release authority acknowledges an interrupted
			// owning cleanup step; a completed free must not reread capacity.
			// This is Go continuation, not another C++ destructor invocation.
			return false, z.Del(a)
		}
		if c.Released {
			return true, arena.ErrInvalidSlot
		}
		if _, e := c.Backing.ReadWord(z.Allocator, 4); e != nil {
			return true, e
		}
	}
	return false, z.Del(a)
}
func (m *RationalManager) cleanupOuter(source bool) error {
	roots := [9]*Int{&m.NTmp, &m.AddTmp1, &m.AddTmp2, &m.LtTmp1.Num, &m.LtTmp1.Den, &m.LtTmp2.Num, &m.LtTmp2.Den, &m.AddMulTmp.Num, &m.AddMulTmp.Den}
	for m.cleanupStage < 9 {
		var e error
		if source {
			var guard bool
			guard, e = sourceNumeralDel(m.Z, roots[m.cleanupStage])
			if guard {
				m.sourceGuard = true
				m.sourceCause = e
			}
		} else {
			e = m.Z.Del(roots[m.cleanupStage])
		}
		if e != nil {
			return e
		}
		m.cleanupStage++
	}
	return nil
}

// SourceFinalize preserves surviving caller direct blocks and all charged cold
// ownership metadata. CloseChecked is the separate owning cleanup operation.
func (m *RationalManager) SourceFinalize() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.fusedExecuting || m.hwfExecuting || m.Z != nil && (m.Z.numericCallerExecuting() || m.Z.rationalServiceReadCallbackActive() || m.Z.conversionCallerExecuting() || m.Z.CopyAllocation.Executing || m.Z.NormalizationExecuting) || (m.mpfParent != nil && m.mpfParent.formatInvocationExecuting()) || m.mpfLifecycleCallbackActive() || (m.Z != nil && m.Z.mpfBorrowedSourceCallbackActive()) {
		return ErrNumericPending
	}
	if m.closed || m.SourceFinalized {
		return nil
	}
	if m.originalCause != nil || (m.Closing && !m.SourceFinalizing) {
		return arena.ErrClosed
	}
	if m.parseOuterPending || m.parseSuffixPending || m.parseB.pending || m.parseExp.pending || m.parseTen.pending || m.prevPowerPending || m.addIntPending || m.fusedPending || m.hwfPending || m.lexical.pending() || m.RawArithmetic.Pending() {
		return ErrRationalLexicalPending
	}
	m.SourceFinalizing = true
	if m.sourceGuard {
		return m.sourceCause
	}
	fail := func(e error) error { m.sourceCause = e; m.CleanupCauseField = e; return e }
	if e := m.finishDecimalStreamSource(); e != nil {
		return fail(e)
	}
	if e := m.finishSMT2StreamSource(); e != nil {
		return fail(e)
	}
	if e := m.finishOwnedStringSource(); e != nil {
		return fail(e)
	}
	if e := m.cleanupOuter(true); e != nil {
		return fail(e)
	}
	if e := m.Z.SourceFinalize(); e != nil {
		if m.Z.sourceGuard {
			m.sourceGuard = true
		}
		return fail(e)
	}
	m.SourceFinalized = true
	m.sourceCause = nil
	m.CleanupCauseField = nil
	return nil
}
