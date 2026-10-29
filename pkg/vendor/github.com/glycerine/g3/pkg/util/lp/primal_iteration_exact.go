package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:841-853,1050-1060;
// lp_primal_core_solver.h:524-527. Precise specialization.

func (m *PairPrimalCoreSolver) NumberOfNonBasicColumnsToTryForEnter() (uint32, error) {
	if err := m.ready(); err != nil {
		return 0, err
	}
	n := uint32(len(*m.NBasis))
	if m.Status == StatusTentativeUnbounded {
		return n, nil
	}
	if n > 300 {
		n = n * m.Settings.PercentOfEnteringToCheck / 100
	}
	if n == 0 {
		return 0, nil
	}
	n = m.Settings.RandomNext() % n
	if n < 1 {
		n = 1
	}
	return n, nil
}

func (m *PairPrimalCoreSolver) DecideOnStatusWhenCannotFindEntering() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled && m.NeedToSwitchCosts() {
		return ErrAssertion
	}
	m.Status = StatusInfeasible
	if m.CurrentXIsFeasible() {
		m.Status = StatusOptimal
	}
	return nil
}

func (m *PairPrimalCoreSolver) OneIteration() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	n, e := m.NumberOfNonBasicColumnsToTryForEnter()
	if e != nil {
		return e
	}
	entering, e := m.ChooseEnteringColumn(n)
	if e != nil {
		return e
	}
	if entering == -1 {
		return m.DecideOnStatusWhenCannotFindEntering()
	}
	return m.AdvanceOnEntering(entering)
}
