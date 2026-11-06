package lp

// Source: src/util/lp/lp_primal_core_solver_tableau.h:323-346;
// lp_primal_core_solver.h:864-874,894-899; retained precise specialization.

func (m *RationalPrimalCoreSolver) BasisColumnIsSetCorrectly(j uint32) (bool, error) {
	if err := m.ready(); err != nil {
		return false, err
	}
	if uint64(j) >= uint64(len(m.A.Columns)) {
		return false, ErrBounds
	}
	return len(m.A.Columns[j]) == 1, nil
}

func (m *RationalPrimalCoreSolver) BasisColumnsAreSetCorrectly() (bool, error) {
	if err := m.ready(); err != nil {
		return false, err
	}
	for _, j := range *m.Basis {
		ok, err := m.BasisColumnIsSetCorrectly(j)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

func (m *RationalPrimalCoreSolver) InitTableauRows() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	m.BlandModeTableau = false
	if err = m.LeftBasisTableau.Clear(); err != nil {
		return err
	}
	if err = m.LeftBasisTableau.Resize(m.A.ColumnCount()); err != nil {
		return err
	}
	m.LeftBasisRepeated = 0
	return nil
}

// InitRunTableau validates existing infeasibility and reduced costs. The source
// does not recompute either here and does not reset the active cost mode.
func (m *RationalPrimalCoreSolver) InitRunTableau() (err error) {
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
		ok, e := m.BasisColumnsAreSetCorrectly()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
	}
	m.BasisSortCounter = 0
	m.TotalIterations = 0
	m.IterationsNoCostGrowth = 0
	if sourceDebugEnabled {
		ok, e := m.InfSetIsCorrect()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
	}
	if m.CurrentXIsFeasible() && m.LookForFeasibleSolutionOnly {
		return nil
	}
	if m.Settings.BackupCosts {
		if err = m.BackupAndNormalizeCosts(); err != nil {
			return err
		}
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	err = m.manager.SetInt32(&m.EpsilonOfReducedCost, 0)
	if err = m.calls.after(m.manager, before, err); err != nil {
		return err
	}
	if m.Settings.UseBreakpointsInFeasibilitySearch {
		if err = m.BreakpointQueue.Resize(m.A.ColumnCount()); err != nil {
			return err
		}
	}
	if m.Settings.SimplexStrategy == StrategyTableauRows {
		if err = m.InitTableauRows(); err != nil {
			return err
		}
	}
	if sourceDebugEnabled {
		ok, e := m.ReducedCostsAreCorrectTableau()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
		pivot, e := m.NeedToPivotToBasisTableau()
		if e != nil {
			return e
		}
		if pivot {
			return ErrAssertion
		}
	}
	return nil
}
