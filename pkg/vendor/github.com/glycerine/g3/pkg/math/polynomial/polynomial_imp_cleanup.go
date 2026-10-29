package polynomial

// Source: src/math/polynomial/polynomial.cpp:1821-1850,2346-2370;
// src/util/vector.h.

import ()

// Returns actual named owning fields; a directory is never swept to recover
// caller values. Repeat traversal is cold and allocation-free.
func (m *MultivariatePolynomialManager) privatePolynomial(index uint32) (Polynomial, bool, error) {
	c := m.core
	roots := [2]Polynomial{c.zero, c.unit}
	for i, p := range roots {
		if c.sourceStage > uint8(i) {
			continue
		}
		if index == 0 {
			return p, true, nil
		}
		index--
	}
	for _, n := range m.newton.children {
		if n == nil || n.closed {
			continue
		}
		fixed := [7]Polynomial{n.scope.temp.value, n.scope.temp.incoming, n.scope.auxPoly.value, n.scope.auxPoly.incoming, n.scope.u.value, n.scope.u.incoming, n.vs.incoming}
		for _, p := range fixed {
			if index == 0 {
				return p, true, nil
			}
			index--
		}
		size := uint32(len(n.vs.values))
		for j := n.vs.drain; j < size; j++ {
			if index == 0 {
				p, e := n.vs.get(c, j)
				return p, true, e
			}
			index--
		}
	}
	return Polynomial{}, false, nil
}
func (m *MultivariatePolynomialManager) callerGuard() error {
	if m.core.arithmeticExecuting() {
		return ErrPolynomialPending
	}
	if m.originalCause != nil {
		return nil
	} // an unreturned failed constructor has no callers.
	distinct, e := m.core.factoryCallerGuard()
	if e != nil {
		return e
	}
	for i := uint32(0); ; i++ {
		p, ok, e := m.privatePolynomial(i)
		if e != nil {
			return e
		}
		if !ok {
			break
		}
		if p == (Polynomial{}) {
			continue
		}
		earlier := false
		count := uint32(0)
		for j := uint32(0); ; j++ {
			q, ok, e := m.privatePolynomial(j)
			if e != nil {
				return e
			}
			if !ok {
				break
			}
			if q == p {
				count++
				if j < i {
					earlier = true
				}
			}
		}
		if earlier {
			continue
		}
		l, e := m.core.polynomialLocation(p)
		if e != nil {
			return e
		}
		if l.deleteStage == 0 {
			rc, e := m.core.readWord(p, 0)
			if e != nil {
				return e
			}
			if rc > count {
				return ErrPolynomialLive
			}
		}
		distinct++
	}
	if m.core.live > distinct {
		return ErrPolynomialLive
	}
	return nil
}
func (m *MultivariatePolynomialManager) CloseChecked() error {
	if m != nil && m.self == m && m.core.nonzeroConstantPending() {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.destructorTraceRunning {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.core.cacheExecuting() || m.core.sparseExecuting(nil)) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m {
		if e := m.core.pendingCacheCause(); e != nil {
			return e
		}
		if e := m.core.pendingSparseCause(); e != nil {
			return e
		}
	}
	if m != nil && m.self == m && m.algebraicIsolationBorrowers != 0 {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.algebraicEvaluation.active || m.algebraicEvaluation.executing) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.factorInline.manager != nil && m.factorInline.manager.matchesFactorConstantInvocation(&m.factorInline) && m.factorInline.frame.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.factorPrelude.active || m.factorEnd.active) {
		return ErrPolynomialPending
	}
	if m != nil && m.matchesFactorCore(&m.factorRoot) && m.factorRoot.frame.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.core != nil && m.core.self == m.core && m.core.integer != nil && (m.core.factorConstantExecuting() || numericIntegerManager_factorNumeralVectorCallerExecuting(m.core.integer) || numericIntegerManager_factorUnivariateCallerExecuting(m.core.integer) || numericIntegerManager_factorCoreCallerExecuting(m.core.integer)) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.core != nil && m.core.self == m.core && (m.core.quasiResultant.active || m.core.variableNumeral.active) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.core != nil && m.core.self == m.core && (m.core.collectingVariables || m.core.algebraicTraceRunning || m.core.renameTraceRunning || m.core.gcdTraceRunning || m.core.dividesTrace.running || m.core.quadraticTraceCallback() || m.core.evaluationTraceCallback()) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.core != nil && m.core.self == m.core && m.core.normalizationPending() {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.core != nil && m.core.self == m.core && (m.core.variableLocal.active || m.core.pscClassic.active || m.core.pscClassic.executing) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.discriminant.active || m.subresultant.active || m.pscChain1.active || m.pscChain2.active) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.core != nil && m.core.self == m.core && m.core.negativeActive && m.core.negativeExecuting {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.core != nil && m.core.displayPending() {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.conversion.active || m.conversionHasOrphans()) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.toPolynomial.active || m.factorNumeralVector.active || m.factorUnivariate.active) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.gcdPending() || m.iccpPending()) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.cra.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.core != nil && m.core.dividesLocal.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.normalization.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.composition.active || m.compositionHasOrphans()) {
		return ErrPolynomialPending
	}

	if m != nil && m.self == m && m.core != nil && m.core.contentLocal.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.monic.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.evaluation.active {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && (m.derivative.active || m.derivativeHasOrphans()) {
		return ErrPolynomialPending
	}
	if e := m.identity(); e != nil {
		return e
	}
	if m.squareRoot.active {
		return ErrPolynomialPending
	}
	if m.core != nil && (m.core.translation.active || m.core.pscPending()) {
		return ErrPolynomialPending
	}
	if len(m.derivative.orphans) > 0 {
		if e := m.CleanupDerivative(); e != nil {
			return e
		}
	}
	if !m.closing {
		if m.core != nil {
			if e := m.callerGuard(); e != nil {
				return e
			}
		}
		m.closing = true
	}
	if cap(m.conversion.orphans) > 0 {
		if e := m.CleanupConversion(); e != nil {
			return e
		}
	}
	if e := m.closeGCDMetadata(); e != nil {
		return e
	}
	if e := m.closeEvaluationFrames(); e != nil {
		return e
	}
	if e := m.closeSquareRootMetadata(); e != nil {
		return e
	}
	if e := m.closeCompositionMetadata(); e != nil {
		return e
	}
	if m.core == nil {
		m.closeToPolynomialHistory()
		m.closeFactorNumeralVectorHistory()
		m.closeFactorUnivariateHistory()
		m.closeFactorCoreHistory()
		m.originalBudget.Release(m.header)
		m.header = 0
		m.closed = true
		return nil
	}
	for m.cleanupStage < 12 {
		var e error
		switch m.cleanupStage {
		case 0:
			if m.constructed >= 5 {
				e = m.core.translateBuffers.CloseChecked()
			}
		case 1:
			if m.constructed >= 5 {
				e = m.core.substitution.positions.CloseChecked()
			}
		case 2:
			if m.constructed >= 5 {
				e = m.core.substitution.max.CloseChecked()
			}
		case 3:
			if m.constructed >= 5 {
				e = m.skeletons.CloseChecked()
			}
		case 4:
			if m.constructed >= 5 {
				e = m.newton.Close()
			}
		case 5:
			if m.constructed >= 5 {
				e = m.iccp.CloseChecked()
			}
		case 6:
			m.varDegrees = nil
		case 7:
			if m.constructed >= 5 {
				e = m.core.substitution.found.close()
			}
		case 8:
			m.lexPermutation = nil
		case 9:
			m.lexBuckets = nil
		case 10:
			m.degree2pos = nil
		case 11:
			m.core.tmp1 = monomialTemp{}
		}
		if e != nil {
			m.cleanupCause = e
			return e
		}
		m.cleanupStage++
	}
	if m.factorService != nil && !m.factorService.closed {
		if e := m.factorService.Close(); e != nil {
			m.cleanupCause = e
			return e
		}
	}
	if m.upm != nil && !m.upm.closed {
		if e := m.upm.Close(); e != nil {
			m.cleanupCause = e
			return e
		}
	}
	// A real body DEC may already have entered/stopped the MM destructor. Only
	// explicit owning continuation acknowledges it; source history remains sticky.
	if (m.mmDropDone || m.mmConstructed) && m.core.monomials != nil && m.core.monomials.managerRefs == 0 && !m.core.monomials.closed {
		if e := m.core.monomials.CloseChecked(); e != nil {
			m.cleanupCause = e
			return e
		}
	}
	if !m.core.closed {
		if e := m.core.CloseChecked(); e != nil {
			m.cleanupCause = e
			return e
		}
	}
	m.closeToPolynomialHistory()
	m.closeFactorNumeralVectorHistory()
	m.closeFactorUnivariateHistory()
	m.closeFactorCoreHistory()
	m.originalBudget.Release(m.header)
	if m.memoryTraceLive && m.completed {
		m.tracePolynomialImpDeallocated()
		m.memoryTraceLive = false
	}
	m.header = 0
	m.closed = true
	m.cleanupCause = nil
	return nil
}
func (m *MultivariatePolynomialManager) Close() {
	if e := m.CloseChecked(); e != nil {
		panic(e)
	}
}
