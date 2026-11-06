package numeral

// Source: src/util/mpq.cpp:28-34; src/util/mpf.cpp:61-67.

import umemory "github.com/glycerine/g3/pkg/util/memory"

func (m *RationalManager) Error() string { return "mpnum: retained rational manager" }
func (m *RationalManager) Unwrap() error {
	if m.originalCause != nil {
		return m.originalCause
	}
	return m.CleanupCauseField
}
func (m *RationalManager) OriginalCause() error { return m.originalCause }
func (m *RationalManager) CleanupCause() error  { return m.CleanupCauseField }
func (m *RationalManager) RuntimeOwner() *umemory.Runtime {
	if m == nil || m.self != m || m.Z == nil {
		return nil
	}
	return m.Z.RuntimeOwner()
}
func (m *RationalManager) Cleanup() error { return m.CloseChecked() }
func (m *RationalManager) CloseChecked() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.fusedExecuting || m.hwfExecuting || m.Z != nil && (m.Z.numericCallerExecuting() || m.Z.rationalServiceReadCallbackActive() || m.Z.conversionCallerExecuting() || m.Z.CopyAllocation.Executing || m.Z.NormalizationExecuting) || (m.mpfParent != nil && m.mpfParent.formatInvocationExecuting()) || m.mpfLifecycleCallbackActive() || (m.Z != nil && m.Z.mpfBorrowedSourceCallbackActive()) {
		return ErrNumericPending
	}
	if m.closed {
		m.CleanupCauseField = nil
		return nil
	}
	if e := m.CleanupHWFConversion(); e != nil {
		m.CleanupCauseField = e
		return m
	}
	if e := m.CleanupParserTemporaries(); e != nil {
		m.CleanupCauseField = e
		return m
	}
	if e := m.CleanupPrevPowerOfTwo(); e != nil {
		m.CleanupCauseField = e
		return m
	}
	if e := m.CleanupAddIntTemporary(); e != nil {
		m.CleanupCauseField = e
		return m
	}
	if e := m.CleanupFusedTemporary(); e != nil {
		m.CleanupCauseField = e
		return m
	}
	if m.SourceFinalizing && !m.SourceFinalized && !m.sourceGuard {
		if e := m.SourceFinalize(); e != nil {
			m.CleanupCauseField = e
			return m
		}
	}
	if e := m.CleanupDecimalStream(); e != nil {
		m.CleanupCauseField = e
		return m
	}
	if e := m.CleanupSMT2Stream(); e != nil {
		m.CleanupCauseField = e
		return m
	}
	if e := m.CleanupOwnedString(); e != nil {
		m.CleanupCauseField = e
		return m
	}
	m.Closing = true
	if e := m.cleanupRawArithmetic(); e != nil {
		m.CleanupCauseField = e
		return m
	}
	if e := m.lexical.cleanup(m.Z); e != nil {
		m.CleanupCauseField = e
		return m
	}
	if !m.sourceGuard {
		if e := m.cleanupOuter(false); e != nil {
			m.CleanupCauseField = e
			return m
		}
	}
	if e := m.Z.CloseChecked(); e != nil {
		m.CleanupCauseField = e
		return m
	}
	// Owning abort releases only child-known roots; stopped source history stays.
	if m.sourceGuard {
		m.NTmp = Int{}
		m.AddTmp1 = Int{}
		m.AddTmp2 = Int{}
		m.LtTmp1 = Rational{}
		m.LtTmp2 = Rational{}
		m.AddMulTmp = Rational{}
	}
	m.BudgetField.Release(m.HeaderBytes)
	m.HeaderBytes = 0
	m.closed = true
	m.CleanupCauseField = nil
	return nil
}
