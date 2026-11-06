package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:675-698, precise branch;
// numeric_pair.h:220-222; util/rational.h:480-488.

// buildAdvanceDelta evaluates sign*t through the scalar-left source overload.
// Its returned pair survives UpdateX; scalar constructor temporaries do not.
func (m *PairPrimalCoreSolver) buildAdvanceDelta(t *RationalPair) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 4)
	m.advanceDelta = NewIndexedPairVector(m.manager, 1)
	sign, tmp, x, y := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2], &m.work.Data[3]
	if err = m.manager.SetInt32(sign, m.SignOfEnteringDelta); err != nil {
		return err
	}
	if err = etaRationalProduct(m.manager, tmp, x, sign, &t.X); err != nil {
		return err
	}
	if err = etaRationalProduct(m.manager, tmp, y, sign, &t.Y); err != nil {
		return err
	}
	if err = m.manager.Copy(&m.advanceDelta.Data[0].X, x); err != nil {
		return err
	}
	if err = m.manager.Copy(&m.advanceDelta.Data[0].Y, y); err != nil {
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
func (m *PairPrimalCoreSolver) AdvanceOnEnteringEqualLeaving(entering int32, t *RationalPair) (err error) {
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
