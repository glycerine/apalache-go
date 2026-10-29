package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1324-1329,432-513.
// Source: src/math/polynomial/upolynomial.cpp:1298-1321.
// Source: src/math/polynomial/algebraic_numbers.cpp:1565-1577; src/util/mpz.h:124-149.
// Source: src/util/mpz.h:allocate; src/util/mpz.cpp scratch/publication callers.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Explicit passed invocation identity. It carries no allocation snapshot and
// creates no source numeral, reservation, or ambient callback owner hook.

type integerAllocationInvocationContext struct {
	squareFreeCall                 *upolySquareFreeCallScope
	squareFreeCallInvocation       uint64
	squareRoot                     *polynomialSqrtInvocationReceipt
	squareRootInvocation           uint64
	defaultNormalization           *upolyDefaultNormalizeScope
	defaultNormalizationInvocation uint64
	berlekamp                      *upolyBerlekampNumeric
	berlekampGeneration            uint64
	sexprArithmetic                *sexprPolynomialArithmeticScope
	sexprArithmeticInvocation      uint64
	sturmIsolation                 *upolySturmIsolationScope
	sturmIsolationInvocation       uint64
	sequenceSturm                  *upolySequenceSturmScope
	sequenceSturmInvocation        uint64
	rootID                         *upolyRootIDScope
	rootIDInvocation               uint64
	knuth                          *upolyKnuthScope
	knuthInvocation                uint64

	rootUpperBound              *upolyRootUpperBoundScope
	removeOneHalfRoot           *upolyRemoveOneHalfRootScope
	rootUpperBoundInvocation    uint64
	removeOneHalfRootInvocation uint64
	oneHalfRoot                 *upolyOneHalfRootScope
	oneHalfRootInvocation       uint64
	integerEvaluation           *upolyIntegerEvaluationScope
	integerEvaluationInvocation uint64

	rationalEvaluation           *upolyRationalEvaluationScope
	rationalEvaluationInvocation uint64
	binaryEvaluation             *upolyBinaryEvaluationScope
	binaryEvaluationInvocation   uint64
	algebraicNegEvalInvocation   uint64
	algebraicNeg                 *algebraicNegScope
	algebraicNegInvocation       uint64
	algebraicNegEval             bool

	upolyDisplay           *upolyDisplayScope
	upolyDisplayInvocation uint64

	factorNumeralVector           *polynomialFactorNumeralVectorReceipt
	factorNumeralVectorInvocation uint64
	toPolynomial                  *polynomialToPolynomialInvocationReceipt
	toPolynomialInvocation        uint64
	factorMemberSet               *polynomialFactorMemberSetReceipt
	factorMemberSetInvocation     uint64
	factorConstant                *polynomialFactorConstantInvocationReceipt
	factorConstantInvocation      uint64
	primePreparation              *upolyFactorPrimePreparationScope
	primePreparationInvocation    uint64
	primePreparationManager       *UPolynomialManager
	preparation                   *upolyFactorPreparationScope
	preparationInvocation         uint64
	trial                         *upolyRecombinationTrialScope
	recombination                 *upolyRecombinationStepScope
	recombinationInvocation       uint64
	recombinationManager          *UPolynomialManager
	somNormalize                  *polynomialSOMNormalizeInvocationReceipt
	somNormalizeInvocation        uint64
	normalization                 *polynomialNormalizeInvocationReceipt
	normalizationInvocation       uint64
	trialInvocation               uint64
	trialManager                  *UPolynomialManager
	henselCaller                  *upolyHenselCallerScope
	henselCallerInvocation        uint64
	henselCallerManager           *UPolynomialManager
	henselPair                    *upolyHenselPairInvocationReceipt
	henselPairInvocation          uint64
	henselManager                 *UPolynomialManager
	iccpConstant                  *polynomialICCPConstantInvocationReceipt
	iccpConstantInvocation        uint64
	primeTrial                    *upolyPrimeTrialInvocationReceipt
	primeTrialInvocation          uint64
	pseudo                        *polynomialPseudoDivisionInvocationReceipt
	pseudoInvocation              uint64
	mignotte                      *upolyMignotteInvocationReceipt
	mignotteInvocation            uint64
	divides                       *polynomialDividesInvocationReceipt
	dividesInvocation             uint64

	neg                   *polynomialNegInvocationReceipt
	negInvocation         uint64
	arithmeticInvocation  uint64
	scalarExact           *polynomialScalarExactDivInvocationReceipt
	scalarExactInvocation uint64
	exactDiv              *polynomialExactDivInvocationReceipt
	exactDivInvocation    uint64
	content               *polynomialContentInvocationReceipt
	contentInvocation     uint64

	arithmetic *polynomialArithmeticInvocationReceipt

	derivative                       *polynomialDerivativeInvocationReceipt
	scalarDiv                        *polynomialScalarDivInvocationReceipt
	factorFrontendConstant           *upolyFactorConstantBinding
	factorFrontendConstantInvocation uint64

	factorFrontend           *upolyFactorFrontendScope
	factorFrontendInvocation uint64
}

func (c integerAllocationInvocationContext) roots(m *unumeral.IntegerManager) *polynomialNumericRoots {
	if c.squareRoot != nil {
		r := c.squareRoot
		if c.squareRootInvocation == 0 || c.squareRootInvocation != r.invocation || r.manager == nil || !r.manager.matchesSqrtInvocation(r) || r.manager.core.integer != m || !r.frame.executing || r.manager.core.callbackDepth != 0 {
			return nil
		}
		return &r.frame.numeric
	}

	if c.knuth != nil && c.knuthFrame(m) == nil {
		return nil
	}
	if q := c.knuthFrame(m); q != nil {
		return &q.numeric
	}

	if c.rootUpperBound != nil && c.rootUpperBoundFrame(m) == nil {
		return nil
	}
	if c.removeOneHalfRoot != nil && c.removeOneHalfRootFrame(m) == nil {
		return nil
	}
	if c.oneHalfRoot != nil && c.oneHalfRootFrame(m) == nil {
		return nil
	}
	if c.integerEvaluation != nil && c.integerEvaluationFrame(m) == nil {
		return nil
	}

	if c.rationalEvaluation != nil && c.rationalEvaluationFrame(m) == nil {
		return nil
	}
	if c.binaryEvaluation != nil && c.binaryEvaluationFrame(m) == nil {
		return nil
	}
	if c.algebraicNeg != nil && c.algebraicNegFrame(m) == nil {
		return nil
	}
	if c.upolyDisplay != nil && c.upolyDisplayFrame(m) == nil {
		return nil
	}

	if c.factorNumeralVector != nil && numericIntegerManager_factorNumeralVectorRoots(m, c) == nil {
		return nil
	}
	if c.toPolynomial != nil && numericIntegerManager_toPolynomialAllocationRoots(m, c) == nil {
		return nil
	}
	if c.factorMemberSet != nil && numericIntegerManager_factorMemberSetRoots(m, c) == nil {
		return nil
	}
	if c.factorConstant != nil && numericIntegerManager_factorConstantContextRoots(m, c) == nil {
		return nil
	}
	if c.factorFrontendConstant != nil && c.factorFrontendConstantFrame(m) == nil {
		return nil
	}

	if c.factorFrontend != nil && c.factorFrontendFrame(m) == nil {
		return nil
	}

	if c.primePreparation != nil && c.primePreparationFrame(m) == nil {
		return nil
	}
	if c.preparation != nil && c.preparationFrame(m) == nil {
		return nil
	}
	if c.recombination != nil && c.recombinationFrame(m) == nil {
		return nil
	}
	if c.trial != nil && c.trialFrame(m) == nil {
		return nil
	}
	if c.henselCaller != nil && c.henselCallerFrame(m) == nil {
		return nil
	}
	if c.henselPair != nil && c.henselPairReceipt(m) == nil {
		return nil
	}
	if c.iccpConstant != nil && numericIntegerManager_iccpConstantContextRoots(m, c) == nil {
		return nil
	}
	if c.primeTrial != nil && c.primeTrialReceipt(m) == nil {
		return nil
	}
	if (c.pseudo != nil && numericIntegerManager_pseudoContextRoots(m, c) == nil) || (c.mignotte != nil && c.mignotteReceipt(m) == nil) {
		return nil
	}

	if c.neg != nil {
		if c.negInvocation == 0 || c.negInvocation != c.neg.invocation || numericIntegerManager_negAllocationRoots(m, c.neg) == nil {
			return nil
		}
	}
	if c.somNormalize != nil && c.somNormalizeReceipt(m) == nil {
		return nil
	}
	if c.normalization != nil && c.normalizationReceipt(m) == nil {
		return nil
	}
	if c.content != nil && c.contentReceipt(m) == nil {
		return nil
	}
	if c.factorMemberSet != nil {
		return numericIntegerManager_factorMemberSetRoots(m, c)
	}
	if c.factorConstant != nil {
		return numericIntegerManager_factorConstantContextRoots(m, c)
	}

	if c.factorNumeralVector != nil {
		return numericIntegerManager_factorNumeralVectorRoots(m, c)
	}
	if c.toPolynomial != nil {
		return numericIntegerManager_toPolynomialAllocationRoots(m, c)
	}

	if q := c.upolyDisplayFrame(m); q != nil {
		return &q.numeric
	}
	if c.iccpConstant != nil {
		return numericIntegerManager_iccpConstantContextRoots(m, c)
	}
	if c.pseudo != nil {
		return numericIntegerManager_pseudoContextRoots(m, c)
	}
	if c.divides != nil {
		return numericIntegerManager_dividesContextRoots(m, c)
	}
	if c.scalarExact != nil {
		return numericIntegerManager_scalarExactContextRoots(m, c)
	}
	if c.exactDiv != nil {
		return numericIntegerManager_exactDivContextRoots(m, c)
	}
	if r := c.somNormalizeReceipt(m); r != nil {
		return numericIntegerManager_somNormalizeAllocationRoots(m, r)
	}
	if p := c.normalizationReceipt(m); p != nil {
		return numericIntegerManager_normalizationAllocationRoots(m, p)
	}
	if p := c.contentReceipt(m); p != nil {
		return numericIntegerManager_contentAllocationRoots(m, p)
	}
	if c.neg != nil {
		return numericIntegerManager_negAllocationRoots(m, c.neg)
	}
	if c.scalarDiv != nil {
		return numericIntegerManager_scalarDivAllocationRoots(m, c.scalarDiv)
	}
	if c.derivative != nil {
		return numericIntegerManager_derivativeAllocationRoots(m, c.derivative)
	}

	if q := c.rootUpperBoundFrame(m); q != nil {
		return &q.numeric
	}
	if q := c.removeOneHalfRootFrame(m); q != nil {
		return &q.numeric
	}
	if q := c.oneHalfRootFrame(m); q != nil {
		return &q.numeric
	}
	if q := c.integerEvaluationFrame(m); q != nil {
		return &q.numeric
	}
	if q := c.rationalEvaluationFrame(m); q != nil {
		return &q.numeric
	}
	if q := c.binaryEvaluationFrame(m); q != nil {
		return &q.numeric
	}
	if q := c.algebraicNegFrame(m); q != nil {
		if c.algebraicNegEval {
			return &q.eval.numeric
		}
		return &q.numeric
	}
	if r := c.primePreparationFrame(m); r != nil {
		return &r.numeric
	}
	if b := c.factorFrontendConstantFrame(m); b != nil {
		if b.record != nil {
			return &b.record.numeric
		}
		return &b.numeric
	}
	if r := c.factorFrontendFrame(m); r != nil {
		return &r.numeric
	}
	if r := c.preparationFrame(m); r != nil {
		return &r.numeric
	}
	if r := c.recombinationFrame(m); r != nil {
		return &r.numeric
	}
	if q := c.trialFrame(m); q != nil {
		return &q.numeric
	}
	if r := c.primeTrialReceipt(m); r != nil {
		return &r.frame.numeric
	}
	if r := c.henselPairReceipt(m); r != nil {
		return &r.frame.numeric
	}
	if r := c.henselCallerFrame(m); r != nil {
		return &r.numeric
	}
	if r := c.mignotteReceipt(m); r != nil {
		return &r.frame.numeric
	}
	return numericIntegerManager_arithmeticAllocationRoots(m, c.arithmeticReceipt(m))
}

func numericIntegerManager_copyFromOwnerWithArithmeticReceipt(m *unumeral.IntegerManager, out *unumeral.Int, owner *unumeral.IntegerManager, value *unumeral.Int, receipt *polynomialArithmeticInvocationReceipt) error {
	return m.WithAllocationContext(((integerAllocationInvocationContext{}).withArithmeticInvocation(receipt)).utility(m)).CopyFromOwnerTracked(out, owner, value, nil)
}
func numericIntegerModularManager_arithmeticReceiptForReceiver(m *unumeral.IntegerModularManager, r *polynomialArithmeticInvocationReceipt) *polynomialArithmeticInvocationReceipt {
	if m == nil || m.Integers() == nil || r == nil || r.manager == nil || r.manager.modular != m || numericIntegerManager_arithmeticAllocationRoots(m.Integers(), r) == nil {
		return nil
	}
	return r
}
func numericIntegerModularManager_arithmeticAllocationContext(m *unumeral.IntegerModularManager, r *polynomialArithmeticInvocationReceipt) integerAllocationInvocationContext {
	return (integerAllocationInvocationContext{}).withArithmeticInvocation(numericIntegerModularManager_arithmeticReceiptForReceiver(m, r))
}
func numericIntegerModularManager_arithmeticAddMulReceipt(m *unumeral.IntegerModularManager, r *polynomialArithmeticInvocationReceipt) *unumeral.IntegerAddMulInvocationReceipt {
	if numericIntegerModularManager_arithmeticReceiptForReceiver(m, r) == nil {
		return nil
	}
	return &r.frame.addMul
}
func numericIntegerModularManager_normalizeWithArithmeticReceipt(m *unumeral.IntegerModularManager, out *unumeral.Int, r *polynomialArithmeticInvocationReceipt) error {
	if numericIntegerModularManager_arithmeticReceiptForReceiver(m, r) == nil {
		return m.PNormalize(out)
	}
	e := m.PNormalizeWithReceipt(out, &r.frame.normalize)
	if r.frame.sourceCause == nil {
		r.frame.sourceCause = nil
	}
	return e
}

func (c integerAllocationInvocationContext) withArithmeticInvocation(r *polynomialArithmeticInvocationReceipt) integerAllocationInvocationContext {
	c.arithmetic = r
	c.arithmeticInvocation = 0
	if r != nil {
		c.arithmeticInvocation = r.invocation
	}
	return c
}
func (c integerAllocationInvocationContext) arithmeticReceipt(m *unumeral.IntegerManager) *polynomialArithmeticInvocationReceipt {
	if c.arithmetic != nil && c.arithmeticInvocation != 0 && c.arithmeticInvocation == c.arithmetic.invocation && numericIntegerManager_arithmeticAllocationRoots(m, c.arithmetic) != nil {
		return c.arithmetic
	}
	return nil
}
