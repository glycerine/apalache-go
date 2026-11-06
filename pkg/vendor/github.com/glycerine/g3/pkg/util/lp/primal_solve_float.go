package lp

// Source: src/util/lp/lp_primal_core_solver.h:524-527;
// lp_primal_core_solver.hpp:841-853,864-963,1050-1060.

func (m *FloatPrimalCoreSolver) NumberOfNonBasicColumnsToTryForEnter() uint32 {
	n := uint32(len(*m.NBasis))
	if m.Status == StatusTentativeUnbounded {
		return n
	}
	if n > 300 {
		n = n * m.Settings.PercentOfEnteringToCheck / 100
	}
	if n == 0 {
		return 0
	}
	n = m.Settings.RandomNext() % n
	if n < 1 {
		n = 1
	}
	return n
}

func (m *FloatPrimalCoreSolver) DecideOnStatusWhenCannotFindEntering() error {
	if sourceDebugEnabled && m.NeedToSwitchCosts() {
		return ErrAssertion
	}
	if m.CurrentXIsFeasible() {
		m.Status = StatusOptimal
	} else {
		m.Status = StatusInfeasible
	}
	return nil
}

func (m *FloatPrimalCoreSolver) OneIteration() error {
	entering, err := m.ChooseEnteringColumn(m.NumberOfNonBasicColumnsToTryForEnter())
	if err != nil {
		return err
	}
	if entering == -1 {
		return m.DecideOnStatusWhenCannotFindEntering()
	}
	return m.AdvanceOnEntering(entering)
}

// Solve executes the retained double stage-one core. Source stopping limits
// use <= after an iteration, and reaching them does not invent a new status.
func (m *FloatPrimalCoreSolver) Solve() (uint32, error) {
	if err := m.InitRun(); err != nil {
		return m.TotalIterations, err
	}
	if m.CurrentXIsFeasible() && m.LookForFeasibleSolutionOnly {
		m.Status = StatusFeasible
		return 0, nil
	}
	bad, err := m.AMultXIsOff()
	if err != nil {
		return m.TotalIterations, err
	}
	if bad {
		m.Status = StatusFloatingPointError
		return 0, nil
	}
	for {
		prefix := "feas"
		if m.UsingInfeasCosts {
			prefix = "inf"
		}
		over, err := m.PrintStatisticsWithIterationsAndNonzeroesAndCostAndCheckTimeOver(&prefix, m.Settings.MessageOut)
		if err != nil || over {
			return m.TotalIterations, err
		}
		if err := m.OneIteration(); err != nil {
			return m.TotalIterations, err
		}
		if sourceDebugEnabled && m.UsingInfeasCosts {
			if err := m.CostsOnNBasisAreZeros(); err != nil {
				return m.TotalIterations, err
			}
		}
		switch m.Status {
		case StatusOptimal, StatusInfeasible:
			if m.LookForFeasibleSolutionOnly {
				break
			}
			if err := m.InitLU(); err != nil {
				return m.TotalIterations, err
			}
			if m.Factorization.GetStatus() != LUOK {
				m.Status = StatusFloatingPointError
				break
			}
			if err := m.InitReducedCosts(); err != nil {
				return m.TotalIterations, err
			}
			entering, err := m.ChooseEnteringColumn(1)
			if err != nil {
				return m.TotalIterations, err
			}
			if entering == -1 {
				if err := m.DecideOnStatusWhenCannotFindEntering(); err != nil {
					return m.TotalIterations, err
				}
				break
			}
			m.Status = StatusUnknown
		case StatusTentativeUnbounded, StatusUnstable:
			if err := m.InitLU(); err != nil {
				return m.TotalIterations, err
			}
			if m.Factorization.GetStatus() != LUOK {
				m.Status = StatusFloatingPointError
				break
			}
			if err := m.InitReducedCosts(); err != nil {
				return m.TotalIterations, err
			}
		case StatusUnbounded:
			if m.CurrentXIsInfeasible() {
				if err := m.InitReducedCosts(); err != nil {
					return m.TotalIterations, err
				}
				m.Status = StatusUnknown
			}
		}
		if m.Status == StatusFloatingPointError || m.Status == StatusUnbounded || m.Status == StatusOptimal || m.Status == StatusInfeasible || m.IterationsNoCostGrowth > m.Settings.MaxIterationsWithoutImprovements || m.TotalIterations > m.Settings.MaxTotalIterations || m.CurrentXIsFeasible() && m.LookForFeasibleSolutionOnly {
			break
		}
	}
	if sourceDebugEnabled && m.Status != StatusFloatingPointError && m.CurrentXIsFeasible() {
		ok, err := m.CalcCurrentXIsFeasibleIncludeNonBasis()
		if err != nil {
			return m.TotalIterations, err
		}
		if !ok {
			return m.TotalIterations, ErrAssertion
		}
	}
	return m.TotalIterations, nil
}
