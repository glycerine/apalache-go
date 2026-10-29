package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:700-768, precise branch;
// util/rational.h scalar comparison and unary minus.

import "github.com/glycerine/g3/pkg/util/numeral"

// Scalar rational >= is !operator<, unlike the pair's lexical expression.
func (m *RationalPrimalCoreSolver) checkAdvanceNonnegative(t *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 1)
	cmp, e := m.manager.Compare(t, &m.work.Data[0])
	if e != nil {
		return e
	}
	if cmp < 0 {
		return ErrAssertion
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}

// Unary -t owns a copied scalar, followed by the separate assignment into t.
func (m *RationalPrimalCoreSolver) negateAdvanceStep(t *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 1)
	neg := &m.work.Data[0]
	if err = m.manager.Copy(neg, t); err != nil {
		return err
	}
	if err = m.manager.Neg(neg); err != nil {
		return err
	}
	if err = m.manager.Copy(t, neg); err != nil {
		return err
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}

// AdvanceOnEnteringAndLeaving consumes the previously solved entering column.
// A nil error can report a source numerical stop through Status. The caller's
// step is mutated only at the source sign-change point.
func (m *RationalPrimalCoreSolver) AdvanceOnEnteringAndLeaving(entering, leaving int32, t *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled {
		if entering < 0 {
			return ErrAssertion
		}
		if len(m.NonBasisList) == 0 {
			return ErrBounds
		}
		if m.NonBasisList[len(m.NonBasisList)-1] != uint32(entering) {
			return ErrAssertion
		}
		if !m.UsingInfeasCosts {
			if t == nil {
				return ErrBounds
			}
			if err = m.checkAdvanceNonnegative(t); err != nil {
				return err
			}
		}
		if leaving < 0 {
			return ErrAssertion
		}
		if entering == leaving {
			if t == nil {
				return ErrBounds
			}
			zero, e := m.manager.IsZero(t)
			if e != nil {
				return e
			}
			if zero {
				return ErrAssertion
			}
		}
	}
	if entering == leaving {
		return m.AdvanceOnEnteringEqualLeaving(entering, t)
	}
	en, lv := uint32(entering), uint32(leaving)
	if uint64(lv) >= uint64(len(*m.Heading)) {
		return ErrBounds
	}
	row := uint32((*m.Heading)[lv])
	if err = m.CalculatePivotRowOfBInverse(row); err != nil {
		return err
	}
	if err = m.CalculatePivotRowWhenInverseReady(row); err != nil {
		return err
	}
	comparison, e := m.PivotsInColumnAndRowAreDifferent(entering, leaving)
	if e != nil {
		return e
	}
	if comparison == 2 {
		m.Status = StatusUnstable
		m.IterationsNoCostGrowth++
		return nil
	}
	if comparison != 0 {
		if sourceDebugEnabled && comparison != 1 {
			return ErrAssertion
		}
		if err = m.InitLU(); err != nil {
			return err
		}
		if m.Factorization == nil || m.Factorization.GetStatus() != LUOK {
			m.Status = StatusUnstable
			m.IterationsNoCostGrowth++
			return nil
		}
	}
	if m.CurrentXIsFeasible() || !m.Settings.UseBreakpointsInFeasibilitySearch {
		if m.SignOfEnteringDelta == -1 {
			if t == nil {
				return ErrBounds
			}
			if err = m.negateAdvanceStep(t); err != nil {
				return err
			}
		}
	}
	ok, e := m.UpdateBasisAndX(entering, leaving, t)
	if e != nil {
		return e
	}
	if !ok {
		if m.Status == StatusFloatingPointError {
			return nil
		}
		if m.LookForFeasibleSolutionOnly {
			m.Status = StatusFloatingPointError
			return nil
		}
		return m.InitReducedCosts()
	}
	zero, e := m.manager.IsZero(t)
	if e != nil {
		return e
	}
	if !zero {
		m.IterationsNoCostGrowth = 0
		if err = m.InitInfeasibilityAfterUpdateXIfInf(lv); err != nil {
			return err
		}
	}
	if m.CurrentXIsFeasible() {
		m.Status = StatusFeasible
		if m.LookForFeasibleSolutionOnly {
			return nil
		}
	}
	if m.NeedToSwitchCosts() {
		err = m.InitReducedCosts()
	} else {
		err = m.UpdateReducedCostsFromPivotRow(en, lv)
	}
	if err != nil {
		return err
	}
	if sourceDebugEnabled && m.NeedToSwitchCosts() {
		return ErrAssertion
	}
	if len(m.NonBasisList) == 0 {
		return ErrBounds
	}
	m.NonBasisList[len(m.NonBasisList)-1] = lv
	return nil
}
