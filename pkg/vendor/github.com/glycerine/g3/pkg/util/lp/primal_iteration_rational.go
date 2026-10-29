package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:1050-1060, precise specialization.

func (m *RationalPrimalCoreSolver) OneIteration() (err error) {
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
