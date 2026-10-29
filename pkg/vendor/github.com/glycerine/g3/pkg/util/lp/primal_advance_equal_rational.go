package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:675-698, precise branch;
// util/rational.h:480-493, concrete scalar mpq/mpq instantiation.

import "github.com/glycerine/g3/pkg/util/numeral"

// buildAdvanceDelta retains the scalar t*sign expression until UpdateX returns.
func (m *RationalPrimalCoreSolver) buildAdvanceDelta(t *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 2)
	m.advanceDelta = NewIndexedRationalVector(m.manager, 1)
	sign, tmp := &m.work.Data[0], &m.work.Data[1]
	if err = m.manager.SetInt32(sign, m.SignOfEnteringDelta); err != nil {
		return err
	}
	if err = etaRationalProduct(m.manager, tmp, &m.advanceDelta.Data[0], t, sign); err != nil {
		return err
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}

// AdvanceOnEnteringEqualLeaving updates values without changing the basis.
// Precise arithmetic never enters the floating residual-repair branch.
func (m *RationalPrimalCoreSolver) AdvanceOnEnteringEqualLeaving(entering int32, t *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled {
		off, e := m.AMultXIsOff()
		if e != nil {
			return e
		}
		if off {
			return ErrAssertion
		}
	}
	if t == nil {
		return ErrBounds
	}
	if err = m.buildAdvanceDelta(t); err != nil {
		return err
	}
	en := uint32(entering)
	if err = m.UpdateX(en, &m.advanceDelta.Data[0]); err != nil {
		return err
	}
	if err = m.advanceDelta.Close(); err != nil {
		return err
	}
	m.advanceDelta = nil
	if _, err = m.AMultXIsOffOnIndex(m.Ed.Index); err != nil {
		return err
	}
	if m.UsingInfeasCosts {
		if sourceDebugEnabled {
			if uint64(en) >= uint64(len(*m.Costs)) {
				return ErrBounds
			}
			zero, e := m.manager.IsZero(&(*m.Costs)[en])
			if e != nil {
				return e
			}
			if !zero {
				return ErrAssertion
			}
		}
		if err = m.InitInfeasibilityCostsForChangedBasisOnly(); err != nil {
			return err
		}
	}
	if m.LookForFeasibleSolutionOnly && m.CurrentXIsFeasible() {
		return nil
	}
	if m.NeedToSwitchCosts() || !m.CurrentXIsFeasible() {
		if err = m.InitReducedCosts(); err != nil {
			return err
		}
	}
	m.IterationsNoCostGrowth = 0
	return nil
}
