package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:243-271;
// upolynomial.cpp:1801-1820 and mpbq.h integer comparison.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

func (m *UPolynomialManager) matchesIntegerEvaluationReceipt(r *upolyIntegerEvaluationInvocationReceipt) bool {
	if m == nil || m.self != m || r == nil {
		return false
	}
	q := &m.integerEvaluation
	return r.owner == m && r.frame == q && q.owner == m && r.invocation != 0 && r.invocation == q.invocation && r.history == q.history
}
func (p *algebraicPredicateScope) integerEvaluationPending(m *AlgebraicRootCore) bool {
	up := m.config.Polynomials
	return up.matchesIntegerEvaluationReceipt(&p.integerEvaluation) && up.integerEvaluation.active
}
func (p *algebraicPredicateScope) cleanupIntegerEvaluation(m *AlgebraicRootCore) error {
	if p.integerEvaluation.owner == nil {
		return nil
	}
	up := m.config.Polynomials
	if e := up.cleanupIntegerEvaluationWithReceipt(&p.integerEvaluation, false); e != nil {
		return e
	}
	return up.cleanupIntegerEvaluationWithReceipt(&p.integerEvaluation, true)
}
func (m *AlgebraicRootCore) lessPredicateInteger(a *unumeral.BinaryRational) (bool, error) {
	p := &m.predicate
	if e := p.beginArithmetic(m); e != nil {
		return false, e
	}
	defer func() { p.arithmeticExecuting = false }()
	return m.config.Binary.WithAllocationContext(p.numericContext()).LessIntegerWithCopyAllocationReceipt(a, &p.candidate, &p.integerComparison)
}
func (m *AlgebraicRootCore) evalPredicateInteger(v UPolynomialView) (int, error) {
	p := &m.predicate
	if e := p.beginArithmetic(m); e != nil {
		return 0, e
	}
	defer func() { p.arithmeticExecuting = false }()
	up := m.config.Polynomials
	sign, e := up.evalSignIntegerWithReceipt(v, &p.candidate, &p.integerEvaluation)
	if up.matchesIntegerEvaluationReceipt(&p.integerEvaluation) && p.childStop == nil {
		p.childStop = up.integerEvaluation.sourceCause
	}
	return sign, e
}

type algebraicIntegerSetInvocation struct {
	owner      *AlgebraicRootCore
	frame      *algebraicIntegerSetScope
	invocation uint64
}

func (m *AlgebraicRootCore) matchesIntegerSetter(r *algebraicIntegerSetInvocation) bool {
	return m != nil && m.self == m && r != nil && r.owner == m && r.frame == &m.integerSet && r.invocation != 0 && r.invocation == m.integerSet.invocation
}
func (m *AlgebraicRootCore) integerSetterPending(r *algebraicIntegerSetInvocation) bool {
	return m.matchesIntegerSetter(r) && m.integerSet.active
}
