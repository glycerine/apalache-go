package lp

// Source: src/util/lp/lar_solver.h:474-542; whole-model scopes.

func (s *LARSolver) Push() (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	s.SimplexStrategy.Value = s.Settings.SimplexStrategy
	s.SimplexStrategy.Push()
	s.Bounds.Status.Push()
	s.Bounds.Witnesses.Push()
	s.Bounds.InfeasibleColumn.Push()
	if err = s.Core.Push(); err != nil {
		return err
	}
	s.TermCount.Value = uint32(len(s.Terms))
	s.TermCount.Push()
	s.ConstraintCount.Value = uint32(len(s.Constraints))
	s.ConstraintCount.Push()
	return nil
}

// Pop follows the source publication order. Numeric children retain their own
// failures; already destroyed tail entries become nil so Close cannot delete
// them twice if a later child stops. A stopped model is only available to Close.
func (s *LARSolver) Pop(k uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	nWas := len(s.ExternalToColumns)
	s.Bounds.Status.Pop(k)
	s.Bounds.InfeasibleColumn.Pop(k)
	n, e := s.Bounds.Witnesses.PeekSize(k)
	if e != nil {
		return e
	}
	for j := nWas; j > int(n); {
		j--
		if j >= len(s.ColumnsToExternal) {
			return ErrBounds
		}
		delete(s.ExternalToColumns, s.ColumnsToExternal[j])
	}
	if uint64(n) > uint64(len(s.ColumnsToExternal)) {
		s.ColumnsToExternal = append(s.ColumnsToExternal, make([]uint32, int(n)-len(s.ColumnsToExternal))...)
	} else {
		s.ColumnsToExternal = s.ColumnsToExternal[:n]
	}
	if s.Settings.UseTableau() {
		if err = s.PopTableau(); err != nil {
			return err
		}
	}
	if err = s.Bounds.Witnesses.Pop(k); err != nil {
		return err
	}
	if err = s.Core.Pop(k); err != nil {
		return err
	}
	if err = CleanLargeElementsAfterPop(n, &s.Bounds.ChangedColumns); err != nil {
		return err
	}
	if err = CleanLargeElementsAfterPop(s.Core.RA.RowCount(), &s.RowsWithChangedBounds); err != nil {
		return err
	}
	if err = s.CleanInfSetOfRSolverAfterPop(); err != nil {
		return err
	}
	if sourceDebugEnabled {
		if s.Settings.SimplexStrategy != StrategyUndecided && s.Settings.UseTableau() {
			correct, e := s.Core.RPrimal.ReducedCostsAreCorrectTableau()
			if e != nil {
				return e
			}
			if !correct {
				return ErrAssertion
			}
		}
		correct, e := s.AxIsCorrect()
		if e != nil {
			return e
		}
		if !correct {
			return ErrAssertion
		}
		correct, e = s.Core.RPrimal.InfSetIsCorrect()
		if e != nil {
			return e
		}
		if !correct {
			return ErrAssertion
		}
	}
	s.ConstraintCount.Pop(k)
	if uint64(s.ConstraintCount.Value) > uint64(len(s.Constraints)) {
		return ErrBounds
	}
	for i := int(s.ConstraintCount.Value); i < len(s.Constraints); i++ {
		if s.Constraints[i] == nil {
			return ErrBounds
		}
		if err = s.Constraints[i].Close(); err != nil {
			return err
		}
		s.Constraints[i] = nil
	}
	s.Constraints = s.Constraints[:s.ConstraintCount.Value]
	s.TermCount.Pop(k)
	if uint64(s.TermCount.Value) > uint64(len(s.Terms)) {
		return ErrBounds
	}
	for i := int(s.TermCount.Value); i < len(s.Terms); i++ {
		if s.Terms[i] == nil {
			return ErrBounds
		}
		if err = s.Terms[i].Close(); err != nil {
			return err
		}
		s.Terms[i] = nil
		if i >= len(s.OriginalTerms) || s.OriginalTerms[i] == nil {
			return ErrBounds
		}
		if err = s.OriginalTerms[i].Close(); err != nil {
			return err
		}
		s.OriginalTerms[i] = nil
	}
	s.Terms = s.Terms[:s.TermCount.Value]
	if uint64(s.TermCount.Value) > uint64(len(s.OriginalTerms)) {
		return ErrBounds
	}
	s.OriginalTerms = s.OriginalTerms[:s.TermCount.Value]
	s.SimplexStrategy.Pop(k)
	s.Settings.SimplexStrategy = s.SimplexStrategy.Value
	if sourceDebugEnabled {
		correct, e := s.SizesAreCorrect()
		if e != nil {
			return e
		}
		if !correct {
			return ErrAssertion
		}
		if s.Settings.UseTableau() {
			correct, e = s.Core.RPrimal.ReducedCostsAreCorrectTableau()
			if e != nil {
				return e
			}
			if !correct {
				return ErrAssertion
			}
		}
	}
	return nil
}
