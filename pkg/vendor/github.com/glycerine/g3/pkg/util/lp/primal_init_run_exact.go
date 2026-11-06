package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:643-663, precise branch.

// InitRun resets run counters, recomputes stored infeasibility, and prepares
// exact costs and sparse-count estimates. A feasible feasibility-only run
// returns before changing the cost mode, backup, epsilon or queue.
func (m *PairPrimalCoreSolver) InitRun() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	m.BasisSortCounter = 0
	m.TotalIterations = 0
	m.IterationsNoCostGrowth = 0
	if err = m.InitInfSet(); err != nil {
		return err
	}
	if m.CurrentXIsFeasible() && m.LookForFeasibleSolutionOnly {
		return nil
	}
	m.UsingInfeasCosts = false
	if m.Settings.BackupCosts {
		if err = m.BackupAndNormalizeCosts(); err != nil {
			return err
		}
	}
	// Capture only this member assignment, ending before delegated owners.
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	err = m.manager.SetInt32(&m.EpsilonOfReducedCost, 0)
	if err = m.calls.after(m.manager, before, err); err != nil {
		return err
	}
	if err = m.BreakpointQueue.Resize(m.A.ColumnCount()); err != nil {
		return err
	}
	if err = m.InitReducedCosts(); err != nil {
		return err
	}
	if uint64(len(m.ColumnsNZ)) != uint64(m.A.ColumnCount()) {
		return m.InitColumnRowNonZeroes()
	}
	return nil
}
