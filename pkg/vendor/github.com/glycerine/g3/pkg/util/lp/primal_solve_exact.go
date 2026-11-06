package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:864-966;
// lp_primal_core_solver_tableau.h:99-208; precise mpq/impq specialization.

// These conditions retain source short-circuit order. Tableau alone adds a
// cancellation read after all these conditions and another read after exit.
func (m *PairPrimalCoreSolver) exactSolveCanContinue() bool {
	return m.Status != StatusFloatingPointError && m.Status != StatusUnbounded &&
		m.Status != StatusOptimal && m.Status != StatusInfeasible &&
		m.IterationsNoCostGrowth <= m.Settings.MaxIterationsWithoutImprovements &&
		m.TotalIterations <= m.Settings.MaxTotalIterations &&
		!(m.CurrentXIsFeasible() && m.LookForFeasibleSolutionOnly)
}

func (m *PairPrimalCoreSolver) refreshExactSolveLU() error {
	if err := m.InitLU(); err != nil {
		return err
	}
	if m.Factorization == nil {
		return ErrBounds
	}
	if m.Factorization.GetStatus() != LUOK {
		m.Status = StatusFloatingPointError
		return nil
	}
	return m.InitReducedCosts()
}

// Solve follows the source precise dispatch, including both LU and tableau
// strategies. Counts on error expose the actual reached iteration prefix.
func (m *PairPrimalCoreSolver) Solve() (count uint32, err error) {
	if err = m.ready(); err != nil {
		return m.TotalIterations, err
	}
	defer func() { m.stopped = err != nil; count = m.TotalIterations }()
	if m.Settings.UseTableau() {
		return m.SolveWithTableau()
	}
	if err = m.InitRun(); err != nil {
		return 0, err
	}
	if m.CurrentXIsFeasible() && m.LookForFeasibleSolutionOnly {
		m.Status = StatusFeasible
		return 0, nil
	}
	for {
		prefix := "feas"
		if m.UsingInfeasCosts {
			prefix = "inf"
		}
		over, e := m.PrintStatisticsWithIterationsAndNonzeroesAndCostAndCheckTimeOver(&prefix, m.Settings.MessageOut)
		if e != nil {
			return 0, e
		}
		if over {
			return 0, nil
		}
		if err = m.OneIteration(); err != nil {
			return 0, err
		}
		if sourceDebugEnabled && m.UsingInfeasCosts {
			ok, e := m.CostsOnNBasisAreZeros()
			if e != nil {
				return 0, e
			}
			if !ok {
				return 0, ErrAssertion
			}
		}
		switch m.Status {
		case StatusOptimal, StatusInfeasible:
			if m.LookForFeasibleSolutionOnly && m.CurrentXIsFeasible() {
				break
			}
			if m.LookForFeasibleSolutionOnly {
				if err = m.InitReducedCosts(); err != nil {
					return 0, err
				}
				entering, e := m.ChooseEnteringColumn(1)
				if e != nil {
					return 0, e
				}
				if entering == -1 {
					if err = m.DecideOnStatusWhenCannotFindEntering(); err != nil {
						return 0, err
					}
					break
				}
				m.Status = StatusUnknown
			}
		case StatusTentativeUnbounded:
			if err = m.refreshExactSolveLU(); err != nil {
				return 0, err
			}
		case StatusUnbounded:
			if m.CurrentXIsInfeasible() {
				if err = m.InitReducedCosts(); err != nil {
					return 0, err
				}
				m.Status = StatusUnknown
			}
		case StatusUnstable:
			if sourceDebugEnabled {
				return 0, ErrAssertion
			}
			if err = m.refreshExactSolveLU(); err != nil {
				return 0, err
			}
		}
		if !m.exactSolveCanContinue() {
			break
		}
	}
	if sourceDebugEnabled && m.Status != StatusFloatingPointError && m.CurrentXIsFeasible() {
		ok, e := m.CalcCurrentXIsFeasibleIncludeNonBasis()
		if e != nil {
			return 0, e
		}
		if !ok {
			return 0, ErrAssertion
		}
	}
	return 0, nil
}

func (m *PairPrimalCoreSolver) SolveWithTableau() (count uint32, err error) {
	if err = m.ready(); err != nil {
		return m.TotalIterations, err
	}
	defer func() { m.stopped = err != nil; count = m.TotalIterations }()
	if err = m.InitRunTableau(); err != nil {
		return 0, err
	}
	if m.CurrentXIsFeasible() && m.LookForFeasibleSolutionOnly {
		m.Status = StatusFeasible
		return 0, nil
	}
	for {
		prefix := "feas t"
		if m.UsingInfeasCosts {
			prefix = "inf t"
		}
		over, e := m.PrintStatisticsWithIterationsAndNonzeroesAndCostAndCheckTimeOver(&prefix, m.Settings.MessageOut)
		if e != nil {
			return 0, e
		}
		if over {
			return 0, nil
		}
		if m.Settings.UseTableauRows() {
			err = m.OneIterationTableauRows()
		} else {
			err = m.OneIterationTableau()
		}
		if err != nil {
			return 0, err
		}
		switch m.Status {
		case StatusOptimal, StatusInfeasible:
			if m.LookForFeasibleSolutionOnly && m.CurrentXIsFeasible() {
				break
			}
			correct, e := m.InfeasibilityCostsAreCorrect()
			if e != nil {
				return 0, e
			}
			if !correct {
				if err = m.InitReducedCostsTableau(); err != nil {
					return 0, err
				}
				entering, e := m.ChooseEnteringColumnTableau()
				if e != nil {
					return 0, e
				}
				if entering == -1 {
					if err = m.DecideOnStatusWhenCannotFindEntering(); err != nil {
						return 0, err
					}
					break
				}
				m.Status = StatusUnknown
			}
		case StatusTentativeUnbounded:
			if err = m.refreshExactSolveLU(); err != nil {
				return 0, err
			}
		case StatusUnbounded:
			if m.CurrentXIsInfeasible() {
				if err = m.InitReducedCosts(); err != nil {
					return 0, err
				}
				m.Status = StatusUnknown
			}
		case StatusUnstable:
			if sourceDebugEnabled {
				return 0, ErrAssertion
			}
			if err = m.refreshExactSolveLU(); err != nil {
				return 0, err
			}
		}
		if !m.exactSolveCanContinue() {
			break
		}
		cancelled, e := m.Settings.CancelFlag()
		if e != nil {
			return 0, e
		}
		if cancelled {
			break
		}
	}
	cancelled, e := m.Settings.CancelFlag()
	if e != nil {
		return 0, e
	}
	if cancelled {
		m.Status = StatusCancelled
	}
	if sourceDebugEnabled && m.Status != StatusFloatingPointError && m.Status != StatusCancelled && m.CurrentXIsFeasible() {
		ok, e := m.CalcCurrentXIsFeasibleIncludeNonBasis()
		if e != nil {
			return 0, e
		}
		if !ok {
			return 0, ErrAssertion
		}
	}
	return 0, nil
}
