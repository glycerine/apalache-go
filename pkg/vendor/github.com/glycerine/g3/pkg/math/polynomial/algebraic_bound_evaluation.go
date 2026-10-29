package polynomial

// Source: algebraic_numbers.cpp:2721-2767 and the reached
// upolynomial.cpp:1734-1764,2757-2814 eval_sign_at calls.

func (m *UPolynomialManager) matchesBinaryEvaluationReceipt(r *upolyBinaryEvaluationInvocationReceipt) bool {
	if m == nil || m.self != m || r == nil {
		return false
	}
	q := &m.binaryEvaluation
	return r.owner == m && r.frame == q && q.owner == m && r.invocation != 0 && r.invocation == q.invocation && r.history == q.history
}
func (s *algebraicRefinementScope) evaluationPending(m *AlgebraicRootCore) bool {
	p := m.config.Polynomials
	return p.matchesBinaryEvaluationReceipt(&s.evaluation) && p.binaryEvaluation.active
}
func (s *algebraicRefinementScope) evaluationSourceCause(m *AlgebraicRootCore) error {
	p := m.config.Polynomials
	if p.matchesBinaryEvaluationReceipt(&s.evaluation) {
		return p.binaryEvaluation.sourceCause
	}
	return nil
}
func (s *algebraicRefinementScope) cleanupEvaluation(m *AlgebraicRootCore) error {
	if s.evaluation.owner == nil {
		return nil
	}
	p := m.config.Polynomials
	if e := p.cleanupBinaryEvaluationWithReceipt(&s.evaluation, false); e != nil {
		return e
	}
	return p.cleanupBinaryEvaluationWithReceipt(&s.evaluation, true)
}

func (s *algebraicBoundScope) evaluationPending(m *AlgebraicRootCore) bool {
	return s.refinement.evaluationPending(m)
}
func (s *algebraicBoundScope) evaluationSourceCause(m *AlgebraicRootCore) error {
	return s.refinement.evaluationSourceCause(m)
}
