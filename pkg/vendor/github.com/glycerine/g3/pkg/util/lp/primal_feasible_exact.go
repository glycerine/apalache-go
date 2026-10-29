package lp

// Source: src/util/lp/lp_core_solver_base.h:541-548;
// lp_primal_core_solver.hpp:1043-1048. Retained mpq/impq specialization.

import "os"

// NonBasicColumnsAreSetCorrectly checks feasibility, including interior boxed
// values. The source does not require every nonbasic value to equal a bound.
func (m *PairCoreSolverBase) NonBasicColumnsAreSetCorrectly() (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	for _, j := range *m.NBasis {
		feasible, e := m.ColumnIsFeasible(j)
		if e != nil {
			return false, e
		}
		if !feasible {
			return false, m.PrintColumnInfo(j, os.Stdout)
		}
	}
	return true, nil
}

func (m *PairPrimalCoreSolver) FindFeasibleSolution() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	m.LookForFeasibleSolutionOnly = true
	if sourceDebugEnabled {
		ok, e := m.NonBasicColumnsAreSetCorrectly()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
	}
	m.Status = StatusUnknown
	_, err = m.Solve()
	return err
}
