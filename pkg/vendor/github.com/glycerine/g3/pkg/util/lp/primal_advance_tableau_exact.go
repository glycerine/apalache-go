package lp

// Source: src/util/lp/lp_primal_core_solver_tableau.h:210-261;
// numeric_pair.h:220-222 and rational scalar multiplication.

func (m *PairPrimalCoreSolver) buildTableauAdvanceDelta(t *RationalPair) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 4)
	m.tableauDelta = NewIndexedPairVector(m.manager, 1)
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
	if err = m.manager.Copy(&m.tableauDelta.Data[0].X, x); err != nil {
		return err
	}
	if err = m.manager.Copy(&m.tableauDelta.Data[0].Y, y); err != nil {
		return err
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}

// AdvanceOnEnteringEqualLeavingTableau constructs the signed full-expression
// argument before updating values. The caller's step stays borrowed and live.
func (m *PairPrimalCoreSolver) AdvanceOnEnteringEqualLeavingTableau(entering int32, t *RationalPair) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.tableauDelta != nil {
		return ErrExactVectorPending
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
	if err = m.buildTableauAdvanceDelta(t); err != nil {
		return err
	}
	if err = m.UpdateXTableau(uint32(entering), &m.tableauDelta.Data[0]); err != nil {
		return err
	}
	if err = m.tableauDelta.Close(); err != nil {
		return err
	}
	m.tableauDelta = nil
	if m.LookForFeasibleSolutionOnly && m.CurrentXIsFeasible() {
		return nil
	}
	if m.NeedToSwitchCosts() {
		if err = m.InitReducedCostsTableau(); err != nil {
			return err
		}
	}
	m.IterationsNoCostGrowth = 0
	return nil
}

// AdvanceOnEnteringAndLeavingTableau retains the source assertion order,
// conditional caller-step negation, zero-step pivot and list publication.
func (m *PairPrimalCoreSolver) AdvanceOnEnteringAndLeavingTableau(entering, leaving int32, t *RationalPair) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.tableauDelta != nil {
		return ErrExactVectorPending
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
		if leaving < 0 || entering < 0 {
			return ErrAssertion
		}
		if m.Settings.SimplexStrategy != StrategyTableauRows {
			if len(m.NonBasisList) == 0 {
				return ErrBounds
			}
			if m.NonBasisList[len(m.NonBasisList)-1] != uint32(entering) {
				return ErrAssertion
			}
		}
		if !m.UsingInfeasCosts {
			if t == nil {
				return ErrBounds
			}
			sign, e := t.Sign(m.manager)
			if e != nil {
				return e
			}
			if sign < 0 {
				return ErrAssertion
			}
		}
		if entering == leaving {
			if t == nil {
				return ErrBounds
			}
			zero, e := t.IsZero(m.manager)
			if e != nil {
				return e
			}
			if zero {
				return ErrAssertion
			}
		}
	}
	if entering == leaving {
		return m.AdvanceOnEnteringEqualLeavingTableau(entering, t)
	}
	if t == nil {
		return ErrBounds
	}
	zero, e := t.IsZero(m.manager)
	if e != nil {
		return e
	}
	en, lv := uint32(entering), uint32(leaving)
	if !zero {
		if (m.CurrentXIsFeasible() || !m.Settings.UseBreakpointsInFeasibilitySearch) && m.SignOfEnteringDelta == -1 {
			if err = m.negateAdvanceStep(t); err != nil {
				return err
			}
		}
		if _, err = m.UpdateBasisAndXTableau(entering, leaving, t); err != nil {
			return err
		}
		if sourceDebugEnabled {
			off, e := m.AMultXIsOff()
			if e != nil {
				return e
			}
			if off {
				return ErrAssertion
			}
		}
		m.IterationsNoCostGrowth = 0
	} else {
		if uint64(lv) >= uint64(len(*m.Heading)) {
			return ErrBounds
		}
		if _, err = m.PivotColumnTableau(en, uint32((*m.Heading)[lv])); err != nil {
			return err
		}
		if err = m.ChangeBasis(en, lv); err != nil {
			return err
		}
	}
	if m.LookForFeasibleSolutionOnly && m.CurrentXIsFeasible() {
		return nil
	}
	if m.Settings.SimplexStrategy != StrategyTableauRows {
		if m.NeedToSwitchCosts() {
			if err = m.InitReducedCostsTableau(); err != nil {
				return err
			}
		}
		if sourceDebugEnabled && m.NeedToSwitchCosts() {
			return ErrAssertion
		}
		if len(m.NonBasisList) == 0 {
			return ErrBounds
		}
		m.NonBasisList[len(m.NonBasisList)-1] = lv
	}
	return nil
}
