package lp

// Source: src/util/lp/lp_core_solver_base.hpp:920-938;
// lp_primal_core_solver.hpp:675-768,784-832.

import (
	"fmt"
	"math"
)

func (m *FloatCoreSolverBase) PivotsInColumnAndRowAreDifferent(entering, leaving int32) (int, error) {
	en, lv := uint32(entering), uint32(leaving)
	if uint64(lv) >= uint64(len(*m.Heading)) {
		return 0, ErrBounds
	}
	i := uint32((*m.Heading)[lv])
	if uint64(i) >= uint64(len(m.Ed.Data)) || uint64(en) >= uint64(len(m.PivotRow.Data)) {
		return 0, ErrBounds
	}
	column, row := m.Ed.Data[i], m.PivotRow.Data[en]
	if column == 0 || row == 0 {
		return 1, nil
	}
	if column < 0 {
		if row > 0 {
			return 2, nil
		}
	} else if row < 0 {
		return 2, nil
	}
	diff := math.Abs((column - row) / (1 + math.Abs(row)))
	if !m.Settings.FloatHarrisSmall(diff / 10) {
		return 1, nil
	}
	return 0, nil
}

func (m *FloatPrimalCoreSolver) AdvanceOnEnteringEqualLeaving(entering int32, t float64) error {
	if sourceDebugEnabled {
		bad, err := m.AMultXIsOff()
		if err != nil {
			return err
		}
		if bad {
			return ErrAssertion
		}
	}
	en := uint32(entering)
	delta := t * float64(m.SignOfEnteringDelta)
	if err := m.UpdateX(en, delta); err != nil {
		return err
	}
	bad, err := m.AMultXIsOffOnIndex(m.Ed.Index)
	if err != nil {
		return err
	}
	if bad {
		ok, err := m.FindXBySolving()
		if err != nil {
			return err
		}
		if !ok {
			if err := m.InitLU(); err != nil {
				return err
			}
			ok, err = m.FindXBySolving()
			if err != nil {
				return err
			}
			if !ok {
				if err := m.RestoreX(en, delta); err != nil {
					return err
				}
				m.IterationsNoCostGrowth++
				if m.Settings.DebugOut != nil {
					_, err := fmt.Fprintf(m.Settings.DebugOut, "failing in advance_on_entering_equal_leaving for entering = %d\n", entering)
					return err
				}
				return nil
			}
		}
	}
	if m.UsingInfeasCosts {
		if sourceDebugEnabled {
			if uint64(en) >= uint64(len(*m.Costs)) {
				return ErrBounds
			}
			if (*m.Costs)[en] != 0 {
				return ErrAssertion
			}
		}
		if err := m.InitInfeasibilityCostsForChangedBasisOnly(); err != nil {
			return err
		}
	}
	if m.LookForFeasibleSolutionOnly && m.CurrentXIsFeasible() {
		return nil
	}
	if m.NeedToSwitchCosts() || !m.CurrentXIsFeasible() {
		if err := m.InitReducedCosts(); err != nil {
			return err
		}
	}
	m.IterationsNoCostGrowth = 0
	return nil
}

func (m *FloatPrimalCoreSolver) AdvanceOnEnteringAndLeaving(entering, leaving int32, t *float64) error {
	if sourceDebugEnabled {
		if entering < 0 {
			return ErrAssertion
		}
		if len(m.NonBasisList) == 0 {
			return ErrBounds
		}
		if m.NonBasisList[len(m.NonBasisList)-1] != uint32(entering) || !m.UsingInfeasCosts && !(*t >= 0) || leaving < 0 || entering == leaving && *t == 0 {
			return ErrAssertion
		}
	}
	if entering == leaving {
		return m.AdvanceOnEnteringEqualLeaving(entering, *t)
	}
	en, lv := uint32(entering), uint32(leaving)
	if uint64(lv) >= uint64(len(*m.Heading)) {
		return ErrBounds
	}
	row := uint32((*m.Heading)[lv])
	if err := m.CalculatePivotRowOfBInverse(row); err != nil {
		return err
	}
	if err := m.CalculatePivotRowWhenInverseReady(row); err != nil {
		return err
	}
	comparison, err := m.PivotsInColumnAndRowAreDifferent(entering, leaving)
	if err != nil {
		return err
	}
	if comparison == 2 {
		m.Status = StatusUnstable
		m.IterationsNoCostGrowth++
		return nil
	}
	if comparison != 0 {
		if err := m.InitLU(); err != nil {
			return err
		}
		if m.Factorization == nil || m.Factorization.GetStatus() != LUOK {
			m.Status = StatusUnstable
			m.IterationsNoCostGrowth++
			return nil
		}
	}
	if err := m.CalcWorkingVectorBetaForColumnNorms(); err != nil {
		return err
	}
	if (m.CurrentXIsFeasible() || !m.Settings.UseBreakpointsInFeasibilitySearch) && m.SignOfEnteringDelta == -1 {
		*t = -*t
	}
	ok, err := m.UpdateBasisAndX(entering, leaving, *t)
	if err != nil {
		return err
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
	if *t != 0 {
		m.IterationsNoCostGrowth = 0
		if err := m.InitInfeasibilityAfterUpdateXIfInf(lv); err != nil {
			return err
		}
	}
	if m.CurrentXIsFeasible() {
		m.Status = StatusFeasible
		if m.LookForFeasibleSolutionOnly {
			return nil
		}
	}
	if err := m.UpdateOrInitColumnNorms(en, lv); err != nil {
		return err
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

func (m *FloatPrimalCoreSolver) AdvanceOnEntering(entering int32) error {
	if sourceDebugEnabled && entering < 0 {
		return ErrAssertion
	}
	en := uint32(entering)
	if err := m.SolveBd(en); err != nil {
		return err
	}
	refresh, err := m.RefreshReducedCostAtEnteringAndCheckThatItIsOff(en)
	if err != nil {
		return err
	}
	if refresh != 0 {
		if m.LookForFeasibleSolutionOnly {
			m.Status = StatusFloatingPointError
			return nil
		}
		if err := m.InitLU(); err != nil {
			return err
		}
		if err := m.InitReducedCosts(); err != nil {
			return err
		}
		if refresh == 2 {
			m.IterationsNoCostGrowth++
			return nil
		}
	}
	t := 0.0
	leaving, err := m.FindLeavingAndT(en, &t)
	if err != nil {
		return err
	}
	if leaving == -1 {
		if !m.CurrentXIsFeasible() {
			if m.Status == StatusUnstable {
				m.Status = StatusFloatingPointError
				return nil
			}
			if err := m.InitInfeasibilityCosts(); err != nil {
				return err
			}
			m.Status = StatusUnstable
			return nil
		}
		if m.Status == StatusTentativeUnbounded {
			m.Status = StatusUnbounded
		} else {
			m.Status = StatusTentativeUnbounded
		}
		return nil
	}
	return m.AdvanceOnEnteringAndLeaving(entering, leaving, &t)
}
