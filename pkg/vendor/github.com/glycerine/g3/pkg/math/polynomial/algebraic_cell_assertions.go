package polynomial

// Source: algebraic_numbers.cpp:383-418,476-504.

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *AlgebraicRootCore) cellAssertionFailure(line int32, expr string) error {
	if e := m.predicateAssertionFailure(line, expr); e != nil {
		return errors.Join(ErrUPolynomialAssertion, e)
	}
	return nil
}
func (m *AlgebraicRootCore) assertCellDegree(count uint32, line int32, expr string, min uint32) error {
	_, enabled, e := algebraicAssertionSwitch()
	if e != nil || !enabled {
		return e
	}
	if count <= min {
		return m.cellAssertionFailure(line, expr)
	}
	return nil
}
func (m *AlgebraicRootCore) assertCellLowerSign(sl int) error {
	_, enabled, e := algebraicAssertionSwitch()
	if e != nil || !enabled {
		return e
	}
	if sl == 0 {
		return m.cellAssertionFailure(386, "sl != 0")
	}
	return nil
}
func (m *AlgebraicRootCore) assertCellUpperSign(v UPolynomialView, upper *unumeral.BinaryRational, sl int) error {
	_, enabled, e := algebraicAssertionSwitch()
	if e != nil || !enabled {
		return e
	}
	sign, e := m.evalCellSign(v, upper, 1)
	if e != nil {
		return e
	}
	if sign != -sl {
		return m.cellAssertionFailure(387, "upm().eval_sign_at(c->m_p_sz, c->m_p, upper(c)) == -sl")
	}
	return nil
}
func (m *AlgebraicRootCore) assertCellNormalizeDegree(r *algebraicRecord) error {
	_, enabled, e := algebraicAssertionSwitch()
	if e != nil || !enabled {
		return e
	}
	if r.size <= 2 {
		return m.cellAssertionFailure(393, "c->m_p_sz > 2")
	}
	return nil
}
func (m *AlgebraicRootCore) assertNewCellFlags(r *algebraicRecord) error {
	_, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled && r.flags>>3 != 0 {
		if e = m.cellAssertionFailure(414, "c->m_i == 0"); e != nil {
			return e
		}
	}
	_, enabled, e = algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled && r.flags&4 != 0 {
		return m.cellAssertionFailure(415, "c->m_not_rational == false")
	}
	return nil
}
func (m *AlgebraicRootCore) assertAssignedCellSign(r *algebraicRecord) error {
	_, enabled, e := algebraicAssertionSwitch()
	if e != nil || !enabled {
		return e
	}
	// The source expression reaches four established to_algebraic accessors.
	for i := 0; i < 4; i++ {
		if e = algebraicQueryTagAssertionSwitch(); e != nil {
			return e
		}
	}
	want := 1
	if r.flags&2 != 0 {
		want = -1
	}
	v, e := m.config.Polynomials.wholeView(r.poly)
	if e != nil {
		return e
	}
	sign, e := m.evalCellSign(v, r.interval.BorrowLower(), 2)
	if e != nil {
		return e
	}
	if sign != want {
		return m.cellAssertionFailure(504, "sign_lower(a.to_algebraic()) == upm().eval_sign_at(a.to_algebraic()->m_p_sz, a.to_algebraic()->m_p, a.to_algebraic()->m_interval.lower())")
	}
	return nil
}
func (m *AlgebraicRootCore) evalCellSign(v UPolynomialView, b *unumeral.BinaryRational, site uint8) (int, error) {
	m.cellEvaluationRunning = true
	defer func() { m.cellEvaluationRunning = false }()
	return m.config.Polynomials.evalSignBinaryWithNegScopeAndReceipt(v, b, nil, &m.cellEvaluation[site])
}
func (m *AlgebraicRootCore) cellEvaluationPending() bool {
	if m.config.Polynomials == nil {
		return false
	}
	for i := range m.cellEvaluation {
		if m.config.Polynomials.matchesBinaryEvaluationReceipt(&m.cellEvaluation[i]) && m.config.Polynomials.binaryEvaluation.active {
			return true
		}
	}
	return false
}
func (m *AlgebraicRootCore) cleanupCellEvaluations() error {
	if m.cellEvaluationRunning {
		return ErrAlgebraicPending
	}
	if m.config.Polynomials == nil {
		return nil
	}
	for i := len(m.cellEvaluation) - 1; i >= 0; i-- {
		if e := m.config.Polynomials.cleanupBinaryEvaluationWithReceipt(&m.cellEvaluation[i], true); e != nil {
			return e
		}
	}
	return nil
}
