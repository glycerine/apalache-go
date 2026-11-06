package polynomial

// Source: src/math/polynomial caller-owned invocation state.

type integerManagerPolynomialBindings struct {
	knuthCaller                  *UPolynomialManager
	rootUpperBoundCaller         *UPolynomialManager
	removeOneHalfRootCaller      *UPolynomialManager
	oneHalfRootCaller            *UPolynomialManager
	integerEvaluationCaller      *UPolynomialManager
	rationalEvaluationCaller     *UPolynomialManager
	binaryEvaluationCaller       *UPolynomialManager
	algebraicNegCaller           *AlgebraicRootCore
	factorCoreCaller             *polynomialFactorCoreFrame
	factorUnivariateCaller       *MultivariatePolynomialManager
	factorNumeralVectorCaller    *MultivariatePolynomialManager
	factorFrontendCaller         *upolyFactorFrontendScope
	upolyDisplayCaller           *upolyDisplayScope
	upolyDisplaySequenceCaller   *UPolynomialSequence
	factorFrontendConstantCaller *UPolynomialFactorizationService
}
type naturalManagerPolynomialBindings struct {
	knuthCaller             *UPolynomialManager
	oneHalfRootCaller       *UPolynomialManager
	rootUpperBoundCaller    *UPolynomialManager
	removeOneHalfRootCaller *UPolynomialManager
}
type integerRawScopePolynomialBindings struct {
	rootUpperBound          *upolyRootUpperBoundScope
	removeOneHalfRoot       *upolyRemoveOneHalfRootScope
	rootUpperInvocation     uint64
	removeOneHalfInvocation uint64
	somNormalize            polynomialSOMNormalizeInvocationReceipt
	normalization           polynomialNormalizeInvocationReceipt
	henselCallerContext     integerAllocationInvocationContext
	henselPair              upolyHenselPairInvocationReceipt
	henselManager           *UPolynomialManager
	divides                 polynomialDividesInvocationReceipt
	content                 polynomialContentInvocationReceipt
	scalarExact             polynomialScalarExactDivInvocationReceipt
	exactDiv                polynomialExactDivInvocationReceipt
}
type integerAddMulScopePolynomialBindings struct {
	parent polynomialArithmeticInvocationReceipt
}
