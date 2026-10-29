package polynomial

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *UPolynomialManager) sequenceOutput(s *UPolynomialSequence) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := s.check(); e != nil {
		return e
	}
	if s.manager != m {
		return ErrUPolynomialOwner
	}
	return nil
}
func (m *UPolynomialManager) SturmSequence(p UPolynomialView, s *UPolynomialSequence) error {
	if e := m.sequenceOutput(s); e != nil {
		return e
	}
	return s.buildSturm(p, UPolynomialView{}, 0)
}
func (m *UPolynomialManager) SturmSequencePair(a, b UPolynomialView, s *UPolynomialSequence) error {
	if e := m.sequenceOutput(s); e != nil {
		return e
	}
	return s.buildSturm(a, b, 1)
}
func (m *UPolynomialManager) SturmTarskiSequence(a, b UPolynomialView, s *UPolynomialSequence) error {
	if e := m.sequenceOutput(s); e != nil {
		return e
	}
	return s.buildSturm(a, b, 2)
}
func (m *UPolynomialManager) FourierSequence(p UPolynomialView, s *UPolynomialSequence) error {
	if e := m.sequenceOutput(s); e != nil {
		return e
	}
	return s.buildSturm(p, UPolynomialView{}, 3)
}
func (m *UPolynomialManager) sequenceVariations(s *upolySequenceState, location uint8, b *unumeral.BinaryRational) (uint32, error) {
	return m.sequenceVariationsWithReceipt(s, location, b, nil)
}
func (m *UPolynomialManager) sequenceVariationsWithReceipt(s *upolySequenceState, location uint8, b *unumeral.BinaryRational, receipt *upolyBinaryEvaluationInvocationReceipt) (uint32, error) {
	if uint32(len(s.sizes)) <= 1 {
		return 0, nil
	}
	var result uint32
	previous := 0
	for i := uint32(0); i < uint32(len(s.sizes)); i++ {
		// Source reads size(i) before coeffs(i). Preserve that assertion/read order.
		n, e := upolyUnsignedRead(s.sizes, i)
		if e != nil {
			return 0, e
		}
		begin, e := upolyUnsignedRead(s.begins, i)
		if e != nil {
			return 0, e
		}
		v := m.stateView(&s.coefficients)
		v.start = begin
		v.count = n
		var sign int
		switch location {
		case 0:
			sign, e = m.EvalSignPlusInfinity(v)
		case 1:
			sign, e = m.EvalSignMinusInfinity(v)
		case 2:
			sign, e = m.EvalSignZero(v)
		case 3:
			sign, e = m.evalSignBinaryWithNegScopeAndReceipt(v, b, nil, receipt)
		}
		if e != nil {
			return 0, e
		}
		if sign == 0 {
			continue
		}
		if e := m.sturmVariationSignAssertion(sign); e != nil {
			return 0, e
		}
		if sign != previous && previous != 0 {
			result++
		}
		previous = sign
	}
	return result, nil
}
func (m *UPolynomialManager) SignVariationsMinusInfinity(s *UPolynomialSequence) (uint32, error) {
	if e := m.sequenceOutput(s); e != nil {
		return 0, e
	}
	return m.sequenceVariations(&s.state, 1, nil)
}
func (m *UPolynomialManager) SignVariationsPlusInfinity(s *UPolynomialSequence) (uint32, error) {
	if e := m.sequenceOutput(s); e != nil {
		return 0, e
	}
	return m.sequenceVariations(&s.state, 0, nil)
}
func (m *UPolynomialManager) SignVariationsZero(s *UPolynomialSequence) (uint32, error) {
	if e := m.sequenceOutput(s); e != nil {
		return 0, e
	}
	return m.sequenceVariations(&s.state, 2, nil)
}
func (m *UPolynomialManager) SignVariationsBinary(s *UPolynomialSequence, b *unumeral.BinaryRational) (uint32, error) {
	if e := m.sequenceOutput(s); e != nil {
		return 0, e
	}
	return s.signVariationsBinary(b)
}
func (m *UPolynomialManager) GetRootID(p UPolynomialView, b *unumeral.BinaryRational) (uint32, error) {
	return m.getRootIDWithReceipt(p, b, nil)
}
func (m *UPolynomialManager) getRootIDWithReceipt(p UPolynomialView, b *unumeral.BinaryRational, receipt *upolyRootIDInvocationReceipt) (value uint32, cause error) {
	q, e := m.beginRootID(p, receipt)
	if e != nil {
		return 0, e
	}
	defer func() { cause = q.finish(cause) }()
	if e = q.sturm(p); e != nil {
		return 0, e
	}
	return q.evaluate(b)
}
func (m *UPolynomialManager) beginRootID(p UPolynomialView, receipt *upolyRootIDInvocationReceipt) (*upolyRootIDScope, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	q := &m.rootID
	if q.active || q.invocation == ^uint64(0) {
		return nil, ErrUPolynomialCleanupPending
	}
	*q = upolyRootIDScope{owner: m, invocation: q.invocation + 1, active: true, executing: true, target: p}
	q.hooks.Policy = q
	q.hooks.Generation = q.invocation
	q.live[2] = true
	if receipt != nil {
		*receipt = upolyRootIDInvocationReceipt{m, q, q.invocation}
	}
	return q, nil
}
func (q *upolyRootIDScope) finish(cause error) error {
	defer func() { q.executing = false }()
	return q.finishVector(2, cause)
}
func (q *upolyRootIDScope) evaluate(b *unumeral.BinaryRational) (uint32, error) {
	m := q.owner
	s0, e := m.sequenceVariations(&q.sequence, 1, nil)
	if e != nil {
		return 0, e
	}
	return q.evaluateLower(b, s0)
}

// The endpoint is reached only after the minus-infinity variation count.
func (q *upolyRootIDScope) evaluateLower(b *unumeral.BinaryRational, s0 uint32) (uint32, error) {
	m := q.owner
	s1, e := m.sequenceVariationsWithReceipt(&q.sequence, 3, b, &q.evaluation)
	if m.matchesBinaryEvaluationReceipt(&q.evaluation) && q.childStop == nil {
		q.childStop = m.binaryEvaluation.sourceCause
	}
	if e != nil {
		return 0, e
	}
	if e = refinementSourceAssert(s0 >= s1, 2956, "s0 >= s1"); e != nil {
		return 0, e
	}
	return s0 - s1, nil
}

// Source: upolynomial.cpp:1877, after skipping zero and before counting.
func (m *UPolynomialManager) sturmVariationSignAssertion(sign int) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil || !enabled {
		return e
	}
	if sign != 1 && sign != -1 {
		return m.scalarDivisionAssertionFailure(1877, "sign == 1 || sign == -1")
	}
	return nil
}
