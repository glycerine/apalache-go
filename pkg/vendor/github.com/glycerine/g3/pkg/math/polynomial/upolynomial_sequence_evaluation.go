package polynomial

// Source: upolynomial.cpp:1845-1903; public sequence's reached binary Horner call.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

func (s *UPolynomialSequence) evaluationPending() bool {
	return s != nil && s.self == s && s.manager != nil && s.manager.matchesBinaryEvaluationReceipt(&s.evaluation) && s.manager.binaryEvaluation.active
}
func (s *UPolynomialSequence) mutationPending() bool {
	return s != nil && s.self == s && (s.displaying || s.evaluating || s.evaluationPending() || s.sturm.active)
}

// CleanupEvaluation acknowledges only the Horner invocation accepted by this
// sequence. A refused call cannot acquire an older public evaluation.
func (s *UPolynomialSequence) CleanupEvaluation() error {
	if e := s.check(); e != nil {
		return e
	}
	if s.evaluating || s.displaying {
		return ErrUPolynomialCleanupPending
	}
	if e := s.manager.cleanupBinaryEvaluationWithReceipt(&s.evaluation, false); e != nil {
		return e
	}
	return s.manager.cleanupBinaryEvaluationWithReceipt(&s.evaluation, true)
}
func (m *UPolynomialManager) sequenceMutationOutput(s *UPolynomialSequence) error {
	if e := m.sequenceOutput(s); e != nil {
		return e
	}
	if s.mutationPending() {
		return ErrUPolynomialCleanupPending
	}
	return nil
}
func (s *UPolynomialSequence) signVariationsBinary(b *unumeral.BinaryRational) (uint32, error) {
	if s.mutationPending() {
		return 0, ErrUPolynomialCleanupPending
	}
	s.evaluating = true
	s.evaluation = upolyBinaryEvaluationInvocationReceipt{}
	s.evaluationSourceStop = nil
	defer func() {
		s.evaluating = false
		if s.manager.matchesBinaryEvaluationReceipt(&s.evaluation) {
			s.evaluationSourceStop = s.manager.binaryEvaluation.sourceCause
		}
	}()
	return s.manager.sequenceVariationsWithReceipt(&s.state, 3, b, &s.evaluation)
}
