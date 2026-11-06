package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1324-1329,432-513.
// Source: src/math/polynomial/upolynomial.cpp:1298-1321.
// Source: src/math/polynomial/algebraic_numbers.cpp:1565-1577; src/util/mpz.h:124-149.
// Source: src/util/mpz.h:124-149; src/util/buffer.h:39-46;
// src/math/polynomial/upolynomial.h:401-450.
// Adapter diagnostic records are materialized only at reached allocations.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c integerAllocationInvocationContext) prepareFactorHistory(z *unumeral.IntegerManager) error {
	if q := c.oneHalfRootFrame(z); q != nil {
		return q.prepareHistory()
	}
	if q := c.knuthFrame(z); q != nil {
		return q.prepareHistory()
	}
	if q := c.rootUpperBoundFrame(z); q != nil {
		return q.prepareHistory()
	}
	if q := c.removeOneHalfRootFrame(z); q != nil {
		return q.prepareHistory()
	}
	if q := c.integerEvaluationFrame(z); q != nil {
		return q.prepareHistory()
	}
	if q := c.rationalEvaluationFrame(z); q != nil {
		return q.prepareHistory()
	}
	if q := c.binaryEvaluationFrame(z); q != nil {
		return q.prepareHistory()
	}
	if q := c.algebraicNegFrame(z); q != nil {
		return q.prepareHistory()
	}
	if r := c.factorNumeralVector; r != nil && numericIntegerManager_factorNumeralVectorRoots(z, c) != nil {
		return r.manager.archiveFactorNumeralVector()
	}
	if r := c.normalizationReceipt(z); r != nil {
		return prepareConverterParentHistory(z, &r.frame.converterHistoryParent)
	}
	if r := c.somNormalizeReceipt(z); r != nil {
		return r.manager.prepareNormalizationConverterHistoryAtAllocation(nil, r)
	}
	if r := c.factorConstant; r != nil && numericIntegerManager_factorConstantContextRoots(z, c) != nil {
		return r.manager.archiveFactorConstantHistory()
	}
	if r := c.factorMemberSet; r != nil && numericIntegerManager_factorMemberSetRoots(z, c) != nil {
		f, s := r.owner, r.frame
		if s.historyRecorded {
			return nil
		}
		s.historyIndex, s.historyRecorded = len(f.memberSetHistory), true
		f.memberSetHistory = append(f.memberSetHistory, polynomialFactorMemberSetHistory{s.invocation, s.normalize, s.bodyCause, s.sourceCause, s.cleanupCause})
	}
	return nil
}
func (f *PolynomialFactors) updateFactorConstantHistory() {
	s := &f.constantInvocation
	if s.historyRecorded {
		f.constantHistory[s.historyIndex] = polynomialFactorConstantHistory{s.invocation, s.kind, s.normalize, s.traceNumeral, s.bodyCause, s.sourceCause, s.cleanupCause}
	}
}
func (f *PolynomialFactors) updateFactorMemberSetHistory() {
	s := &f.memberSet
	if s.historyRecorded {
		f.memberSetHistory[s.historyIndex] = polynomialFactorMemberSetHistory{s.invocation, s.normalize, s.bodyCause, s.sourceCause, s.cleanupCause}
	}
}

func prepareConverterParentHistory(z *unumeral.IntegerManager, r *polynomialFactorNumeralVectorReceipt) error {
	if r.manager == nil {
		return nil
	}
	c := integerAllocationInvocationContext{factorNumeralVector: r, factorNumeralVectorInvocation: r.invocation}
	if numericIntegerManager_factorNumeralVectorRoots(z, c) == nil {
		return ErrPolynomialPending
	}
	return r.manager.archiveFactorNumeralVector()
}

// Diagnostic-only allocation records follow concrete reached normalization
// child tuples. They never acquire NUM, reference, vector or factory authority.
func (c *MultivariatePolynomialCore) prepareNormalizationConverterHistoryAtAllocation(f *polynomialFactoryInvocationReceipt, som *polynomialSOMNormalizeInvocationReceipt) error {
	if c == nil || c.self != c || c.callbackDepth != 0 {
		return nil
	}
	r := c.normalizationOwner
	if r.manager == nil || r.manager.core != c || !r.manager.hasNormalizeInvocationHistory(&r) || !r.frame.active || !r.frame.executing {
		return nil
	}
	s := r.frame
	reached := false
	if f != nil && c.matchesFactoryInvocation(f) && *f == s.factory {
		reached = true
	}
	if som != nil && c.hasSOMNormalizeInvocationHistory(som) && *som == s.som {
		reached = true
	}
	if !reached {
		return nil
	}
	return prepareConverterParentHistory(c.integer, &s.converterHistoryParent)
}
