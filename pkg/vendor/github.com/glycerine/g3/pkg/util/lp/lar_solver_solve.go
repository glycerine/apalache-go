package lp

// Source: src/util/lp/lar_solver.h:433-454,851-869.

func (s *LARSolver) FindFeasibleSolution() (status Status, err error) {
	if err = s.ready(); err != nil {
		return s.Bounds.Status.Value, err
	}
	defer func() { s.stopped = err != nil }()
	if s.StrategyIsUndecided() {
		if err = s.DecideOnStrategyAndAdjustInitialState(); err != nil {
			return s.Bounds.Status.Value, err
		}
	}
	s.Core.RPrimal.LookForFeasibleSolutionOnly = true
	return s.Solve()
}

func (s *LARSolver) Solve() (status Status, err error) {
	if err = s.ready(); err != nil {
		return s.Bounds.Status.Value, err
	}
	defer func() { s.stopped = err != nil }()
	if s.Bounds.Status.Value == StatusInfeasible {
		return s.Bounds.Status.Value, nil
	}
	if err = s.SolveWithCoreSolver(); err != nil {
		return s.Bounds.Status.Value, err
	}
	if s.Bounds.Status.Value != StatusInfeasible && s.Settings.BoundPropagation {
		if err = s.DetectRowsWithChangedBounds(); err != nil {
			return s.Bounds.Status.Value, err
		}
	}
	if err = s.Bounds.ChangedColumns.Clear(); err != nil {
		return s.Bounds.Status.Value, err
	}
	return s.Bounds.Status.Value, nil
}

func (s *LARSolver) SolveWithCoreSolver() (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	c, r := s.Core, s.Core.RPrimal
	if !s.Settings.UseTableau() {
		if err = s.AddLastRowsToPairLU(r); err != nil {
			return err
		}
	}
	if c.NeedToPresolveWithDoubleSolver() {
		if err = s.AddLastRowsToFloatLU(c.DPrimal); err != nil {
			return err
		}
	}
	if err = c.PrefixR(); err != nil {
		return err
	}
	if s.Settings.SimplexStrategy != StrategyTableauRows {
		if err = s.BasicColumnsWithChangedCost.Clear(); err != nil {
			return err
		}
		if err = s.BasicColumnsWithChangedCost.Resize(uint32(len(c.RX.Data))); err != nil {
			return err
		}
	}
	if s.Settings.UseTableau() {
		err = s.UpdateXAndInfCostsForColumnsWithChangedBoundsTableau()
	} else {
		err = s.UpdateXAndInfCostsForColumnsWithChangedBounds()
	}
	if err != nil {
		return err
	}
	if err = c.Solve(); err != nil {
		return err
	}
	s.Bounds.Status.Value = r.Status
	if sourceDebugEnabled && s.Bounds.Status.Value == StatusOptimal {
		var holds bool
		holds, err = s.AllConstraintsHold()
		if err != nil {
			return err
		}
		if !holds {
			return ErrAssertion
		}
	}
	return nil
}
