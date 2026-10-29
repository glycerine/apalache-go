package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1324-1329,432-513.
// Source: src/math/polynomial/upolynomial.cpp:1298-1321.
// Source: src/math/polynomial/algebraic_numbers.cpp:1565-1577; src/math/polynomial/upolynomial.cpp:1734-1764.
// Source: src/math/polynomial/polynomial.cpp:4507-4528;
// src/util/mpz.cpp; src/util/mpzzp.h.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// The passed receipt names the actual full manager's embedded derivative
// frame. Allocation leaves acquire only under its executing source invocation.

func numericIntegerManager_derivativeAllocationRoots(m *unumeral.IntegerManager, r *polynomialDerivativeInvocationReceipt) *polynomialNumericRoots {
	if r == nil || r.manager == nil || r.manager.self != r.manager || r.manager.core == nil || r.manager.core.self != r.manager.core || r.manager.core.integer != m || !r.manager.matchesDerivativeInvocation(r) || !r.frame.executing || r.manager.core.callbackDepth != 0 {
		return nil
	}
	return &r.frame.numeric
}

func numericIntegerModularManager_setUint32WithDerivativeReceipt(m *unumeral.IntegerModularManager, out *unumeral.Int, v uint32, r *polynomialDerivativeInvocationReceipt) error {
	return m.WithAllocationContext((numericIntegerModularManager_derivativeAllocationContext(m, r)).utility(m.Integers())).SetUint32(out, v)
}
func numericIntegerModularManager_mulWithDerivativeReceipt(m *unumeral.IntegerModularManager, a, b, out *unumeral.Int, r *polynomialDerivativeInvocationReceipt) error {
	return m.WithAllocationContext((numericIntegerModularManager_derivativeAllocationContext(m, r)).utility(m.Integers())).Mul(a, b, out)
}
func numericIntegerModularManager_derivativeAllocationContext(m *unumeral.IntegerModularManager, r *polynomialDerivativeInvocationReceipt) integerAllocationInvocationContext {
	if m == nil || m.Integers() == nil || numericIntegerManager_derivativeAllocationRoots(m.Integers(), r) == nil || r.manager.core.modular != m {
		return integerAllocationInvocationContext{}
	}
	return integerAllocationInvocationContext{derivative: r}
}

func numericIntegerModularManager_normalizeWithSourceContext(m *unumeral.IntegerModularManager, out *unumeral.Int, context integerAllocationInvocationContext) error {
	if q := context.oneHalfRootFrame(m.Integers()); q != nil && q.owner.modular == m {
		e := m.WithAllocationContext((context).utility(m.Integers())).PNormalizeWithFactorHistoryTracked(out, &q.normalize)
		return e
	}
	if q := context.knuthFrame(m.Integers()); q != nil && q.owner.modular == m {
		e := m.WithAllocationContext((context).utility(m.Integers())).PNormalizeWithFactorHistoryTracked(out, &q.normalize)
		return e
	}
	if q := context.rootUpperBoundFrame(m.Integers()); q != nil && q.owner.modular == m {
		e := m.WithAllocationContext((context).utility(m.Integers())).PNormalizeWithFactorHistoryTracked(out, &q.normalize)
		return e
	}
	if q := context.removeOneHalfRootFrame(m.Integers()); q != nil && q.owner.modular == m {
		e := m.WithAllocationContext((context).utility(m.Integers())).PNormalizeWithFactorHistoryTracked(out, &q.normalize)
		return e
	}
	if q := context.integerEvaluationFrame(m.Integers()); q != nil && q.owner.modular == m {
		e := m.WithAllocationContext((context).utility(m.Integers())).PNormalizeWithFactorHistoryTracked(out, &q.normalize)
		return e
	}
	if q := context.rationalEvaluationFrame(m.Integers()); q != nil && q.owner.modular == m {
		e := m.WithAllocationContext((context).utility(m.Integers())).PNormalizeWithFactorHistoryTracked(out, &q.normalize)
		return e
	}
	if q := context.binaryEvaluationFrame(m.Integers()); q != nil && q.owner.modular == m {
		e := m.WithAllocationContext((context).utility(m.Integers())).PNormalizeWithFactorHistoryTracked(out, &q.normalize)
		return e
	}
	if q := context.algebraicNegFrame(m.Integers()); q != nil && q.owner.config.Polynomials.modular == m {
		child := &q.normalize
		if context.algebraicNegEval {
			child = &q.eval.normalize
		}
		e := m.WithAllocationContext((context).utility(m.Integers())).PNormalizeWithFactorHistoryTracked(out, child)
		var cause error
		if context.algebraicNegEval {
			if q.eval.sourceCause == nil {
				q.eval.sourceCause = cause
			}
		} else if q.sourceCause == nil {
			q.sourceCause = cause
		}
		return e
	}
	if q := context.upolyDisplayFrame(m.Integers()); q != nil && q.owner.modular == m {
		e := m.PNormalizeWithReceipt(out, &q.normalize)
		return e
	}

	if r := context.factorNumeralVector; r != nil && numericIntegerManager_factorNumeralVectorRoots(m.Integers(), context) != nil && r.frame.outputManager != nil && r.frame.outputManager.modular == m {
		e := m.WithAllocationContext((context).utility(m.Integers())).PNormalizeWithFactorHistoryTracked(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	if r := context.toPolynomial; r != nil && numericIntegerManager_toPolynomialAllocationRoots(m.Integers(), context) != nil && r.manager.core.modular == m {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	if r := context.factorMemberSet; numericIntegerManager_factorMemberSetRoots(m.Integers(), context) != nil && r.owner.manager.core.modular == m {
		e := m.WithAllocationContext((context).utility(m.Integers())).PNormalizeWithFactorHistoryTracked(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	if r := context.factorConstant; numericIntegerManager_factorConstantContextRoots(m.Integers(), context) != nil && r.manager.manager.core.modular == m {
		child := &r.frame.normalize[r.frame.phase]
		e := m.WithAllocationContext((context).utility(m.Integers())).PNormalizeWithFactorHistoryTracked(out, child)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	if b := context.factorFrontendConstantFrame(m.Integers()); b != nil && b.service.manager.modular == m {
		return m.WithAllocationContext((context).utility(m.Integers())).PNormalizeWithFactorHistoryTracked(out, &b.normalize)
	}
	if q := context.factorFrontendFrame(m.Integers()); q != nil && q.service.manager.modular == m {
		e := m.PNormalizeWithReceipt(out, &q.normalize)
		return e
	}
	if q := context.preparationFrame(m.Integers()); q != nil && q.service.manager.modular == m {
		e := m.PNormalizeWithReceipt(out, &q.normalize)
		return e
	}
	if q := context.primePreparationFrame(m.Integers()); q != nil && context.primePreparationManager.modular == m {
		child := q.normalizeReceipt(m)
		e := m.PNormalizeWithReceipt(out, child)
		return e
	}
	if q := context.recombinationFrame(m.Integers()); q != nil && context.recombinationManager.modular == m {
		child := q.normalizeReceipt(m)
		e := m.PNormalizeWithReceipt(out, child)
		return e
	}
	if q := context.trialFrame(m.Integers()); q != nil && context.trialManager.modular == m {
		child := q.normalizeReceipt(m)
		e := m.PNormalizeWithReceipt(out, child)
		return e
	}
	if q := context.henselCallerFrame(m.Integers()); q != nil && context.henselCallerManager.modular == m {
		child := q.normalizeReceipt(m)
		e := m.PNormalizeWithReceipt(out, child)
		return e
	}

	if r := context.henselPairReceipt(m.Integers()); r != nil && context.henselManager.modular == m {
		child := r.frame.normalizeReceipt(m)
		e := m.PNormalizeWithReceipt(out, child)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}

	if r := context.iccpConstant; numericIntegerManager_iccpConstantContextRoots(m.Integers(), context) != nil && r.manager.core.modular == m {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	if r := context.primeTrialReceipt(m.Integers()); r != nil && (r.frame.zp.modular == m || r.service.manager.modular == m) {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	if r := context.pseudo; numericIntegerManager_pseudoContextRoots(m.Integers(), context) != nil && r.manager.modular == m {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	if r := context.mignotteReceipt(m.Integers()); r != nil && r.service.manager.modular == m {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	if r := context.divides; numericIntegerManager_dividesContextRoots(m.Integers(), context) != nil && r.core.modular == m {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	if r := context.neg; r != nil && context.negInvocation != 0 && context.negInvocation == r.invocation && numericIntegerManager_negAllocationRoots(m.Integers(), r) != nil && r.manager.modular == m {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.negativeCleanupCause == nil {
			r.frame.negativeCleanupCause = nil
		}
		return e
	}
	if r := context.scalarExact; numericIntegerManager_scalarExactContextRoots(m.Integers(), context) != nil && r.core.modular == m {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	if r := context.exactDiv; numericIntegerManager_exactDivContextRoots(m.Integers(), context) != nil && r.core.modular == m {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}

	if r := context.somNormalizeReceipt(m.Integers()); r != nil && r.manager.modular == m {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	if r := context.normalizationReceipt(m.Integers()); r != nil && r.manager.core.modular == m {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	if r := context.contentReceipt(m.Integers()); r != nil && r.manager.modular == m {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.cleanupCause == nil {
			r.frame.cleanupCause = nil
		}
		return e
	}
	if r := context.scalarDiv; numericIntegerManager_scalarDivAllocationRoots(m.Integers(), r) != nil && r.manager.modular == m {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	if r := context.derivative; numericIntegerManager_derivativeAllocationRoots(m.Integers(), r) != nil && r.manager.core.modular == m {
		e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
		if r.frame.sourceCause == nil {
			r.frame.sourceCause = nil
		}
		return e
	}
	return numericIntegerModularManager_normalizeWithArithmeticReceipt(m, out, context.arithmeticReceipt(m.Integers()))
}
