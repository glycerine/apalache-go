package polynomial

// Source: src/math/polynomial/polynomial.cpp:1821-1850,2346-2370;
// src/util/vector.h; src/util/mpzzp.h:97-105.

import (
	"github.com/glycerine/g3/pkg/arena"
)

// This records only actual completed member construction. Failed constructors
// never execute the imp destructor body or release the body MM retain.
func (m *MultivariatePolynomialManager) unwindMembers() error {
	if m.core == nil {
		return nil
	}
	return m.destroyMembers()
}
func (m *MultivariatePolynomialManager) destroyMembers() error {
	// All late declarations are default constructed before the body starts.
	if m.constructed >= 5 {
		for m.memberStage < 15 {
			var e error
			switch m.memberStage {
			case 0:
				e = m.core.translateBuffers.SourceFinalize()
			case 1:
				e = m.core.substitution.positions.SourceFinalize()
			case 2:
				e = m.core.substitution.max.SourceFinalize()
			case 3:
				e = m.skeletons.SourceFinalize()
			case 4:
				e = m.newton.sourceFinalizeMember()
			case 5:
				e = m.iccp.SourceFinalize()
			case 6:
				m.varDegrees = nil
			case 7:
				e = m.core.substitution.found.sourceFinalize()
			case 8:
				m.lexPermutation = nil
			case 9:
				m.lexBuckets = nil
			case 10:
				e = m.sourceBufferMembers(&m.core.cheap2, false)
			case 11:
				e = m.sourceBufferMembers(&m.core.cheap, false)
			case 12:
				e = m.sourceBufferMembers(&m.core.som2, true)
			case 13:
				e = m.sourceBufferMembers(&m.core.som, true)
			case 14:
				m.degree2pos = nil
			}
			if e != nil {
				return e
			}
			m.memberStage++
		}
	}
	if m.constructed >= 4 && m.memberStage < 19 {
		for m.memberStage < 19 {
			var e error
			switch m.memberStage {
			case 15:
				// Retain actual orphan handles for explicit owning cleanup; native storage has no source free.
			case 16:
				e = m.sourceNumeralVector(&m.core.linearTerms)
			case 17:
				e = m.sourceNumeralVector(&m.core.rationalTerms)
			case 18:
				m.core.tmp1 = monomialTemp{}
			}
			if e != nil {
				return e
			}
			m.memberStage++
		}
	}
	if m.constructed >= 3 && m.memberStage < 22 {
		for m.memberStage < 22 {
			var e error
			switch m.memberStage {
			case 19:
				m.core.queryPositions.releaseValues()
			case 20:
				m.core.idsSourceDestroyed = true
				m.core.ids.Cleanup(0)
				m.core.idsInitialized = false
			case 21:
				m.core.directorySourceDestroyed = true
				m.core.polynomials = nil
			}
			if e != nil {
				return e
			}
			m.memberStage++
		}
	}
	if m.constructed >= 2 && m.memberStage < 23 {
		if e := m.upm.SourceFinalize(); e != nil {
			return e
		}
		m.memberStage = 23
	}
	if m.constructed >= 1 {
		m.core.modularSourceDestroyed = true
		if e := m.core.modular.FinalizeSourceMembers(&m.modularIndex); e != nil {
			return e
		}
	}
	return nil
}

// Source mpz has no destructor. Native reset retains discarded BIG resources
// in this same owner for explicit owning cleanup without freeing them early.
func (m *MultivariatePolynomialManager) sourceNumeralVector(v *polynomialNumerals) error {
	if e := v.reset(m.core); e != nil {
		return e
	}
	v.values = nil
	return nil
}
func (m *MultivariatePolynomialManager) sourceBufferMembers(b *polynomialSOM, positions bool) error {
	b.ms = nil
	if e := m.sourceNumeralVector(&b.as); e != nil {
		return e
	}
	if positions {
		b.positions.releaseValues()
	}
	return nil
}

func (v *PolynomialNewtonVector) sourceFinalizeMember() error {
	if v.sourceCause != nil {
		return v.sourceCause
	}
	v.sourceFinalizing = true
	if v.sourceStage == 0 {
		if e := v.flushMode(false); e != nil {
			v.sourceCause = e
			return e
		}
		v.sourceStage = 1
	}
	if v.sourceStage == 1 {
		v.children = nil
		v.sourceStage = 2
	}
	v.sourceFinalized = true
	return nil
}

func (m *MultivariatePolynomialManager) SourceFinalize() error {
	if m != nil && m.self == m && m.core.nonzeroConstantPending() {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.core != nil {
		if e := m.core.pendingLexSortCause(); e != nil {
			return e
		}
	}
	if m != nil && m.self == m {
		for _, trace := range [2]*polynomialEvaluationTraceScope{&m.evaluation.trace, &m.algebraicEvaluation.trace} {
			if trace.active || trace.executing {
				if trace.sourceCause != nil {
					return trace.sourceCause
				}
				return ErrPolynomialPending
			}
		}
	}
	if m != nil && m.self == m {
		if m.factorEnd.sourceCause != nil {
			return m.factorEnd.sourceCause
		}
		if m.factorPrelude.sourceCause != nil {
			return m.factorPrelude.sourceCause
		}
		if m.factorEnd.active || m.factorPrelude.active {
			return ErrPolynomialPending
		}
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
	if m != nil && m.self == m && m.factorInline.manager != nil {
		if cause := m.factorInline.manager.factorConstantInvocationSourceCause(&m.factorInline); cause != nil {
			return cause
		}
	}
	if m != nil && m.matchesFactorCore(&m.factorRoot) && !m.factorRoot.frame.executing && !m.factorRoot.frame.finishing && m.factorRoot.frame.sourceCause != nil {
		return m.factorRoot.frame.sourceCause
	}
	if m != nil && m.self == m && m.factorUnivariate.invocation != 0 && !m.factorUnivariate.executing && !m.factorUnivariate.finishing && m.factorUnivariate.sourceCause != nil {
		return m.factorUnivariate.sourceCause
	}
	if m != nil && m.self == m && m.factorNumeralVector.invocation != 0 && !m.factorNumeralVector.executing && !m.factorNumeralVector.finishing && m.factorNumeralVector.sourceCause != nil {
		return m.factorNumeralVector.sourceCause
	}
	if m != nil && m.self == m && m.toPolynomial.active && !m.toPolynomial.executing && !m.toPolynomial.finishing && m.toPolynomial.sourceCause != nil {
		return m.toPolynomial.sourceCause
	}
	if m != nil && m.self == m && (m.core.factorConstantExecuting() || m.core.toPolynomialCallerExecuting() || numericIntegerManager_factorNumeralVectorCallerExecuting(m.core.integer) || numericIntegerManager_factorUnivariateCallerExecuting(m.core.integer) || numericIntegerManager_factorCoreCallerExecuting(m.core.integer)) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.core != nil && m.core.self == m.core && (m.core.quasiResultant.active || m.core.variableNumeral.active) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.core != nil && m.core.self == m.core && (m.core.collectingVariables || m.core.algebraicTraceRunning || m.core.renameTraceRunning || m.core.gcdTraceRunning || m.core.dividesTrace.running || m.core.quadraticTraceCallback() || m.core.evaluationTraceCallback()) {
		return ErrPolynomialPending
	}
	if m != nil && m.self == m && m.core != nil && m.core.self == m.core && (m.normalization.active || m.core.normalizationPending()) {
		return ErrPolynomialPending
	}
	if e := m.identity(); e != nil {
		return e
	}
	if m.discriminant.active || m.subresultant.active || m.pscChain1.active || m.pscChain2.active || m.core.pscClassic.active || m.core.pscClassic.executing {
		return ErrPolynomialPending
	}
	if m.closing || !m.completed || m.originalCause != nil {
		return arena.ErrClosed
	}
	if m.sourceCause != nil {
		return m.sourceCause
	}
	if m.sourceFinalized {
		return nil
	}
	m.sourceStarted = true
	m.core.sourceStarted = true
	for m.bodyStage < 11 {
		var e error
		switch m.bodyStage {
		case 0, 1:
			e = m.core.sourcePrivateDEC(uint32(m.bodyStage))
		case 2:
			e = m.core.som.reset()
		case 3:
			e = m.core.som2.reset()
		case 4:
			e = m.core.cheap.reset()
		case 5:
			e = m.core.cheap2.reset()
		case 6:
			_, e = upolynomialSourceNumeralDel(m.core.integer, &m.core.zeroNumeral)
		case 7:
			e = m.newton.flushMode(false)
		case 8:
			e = m.skeletons.resetSource()
		case 9:
			if polynomialSourceDebug {
				e = m.traceLeakedPolynomials()
				if e != nil {
					break
				}
				clear(m.core.polynomials)
				m.core.polynomials = m.core.polynomials[:0]
			}
			if e == nil {
				e = polynomialImpSourceAssert(2367, "m_polynomials.empty()", len(m.core.polynomials) == 0)
			}
		case 10:
			before := m.core.monomials.managerRefs
			e = m.core.monomials.DecManagerRef()
			// This actual source operand decrements before the child destructor.
			// The owning abort must never repeat that decrement after a child stop.
			m.mmDropDone = m.core.monomials.managerRefs != before
			if m.mmDropDone {
				m.core.monomialRetained = false
			}
		}
		if e != nil {
			m.sourceCause = e
			return e
		}
		m.bodyStage++
		if m.bodyStage <= 7 {
			m.core.sourceStage = m.bodyStage
		}
	}
	if e := m.destroyMembers(); e != nil {
		m.sourceCause = e
		return e
	}
	m.sourceFinalized = true
	return nil
}
