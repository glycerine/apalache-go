package lp

// Source: src/util/lp/lar_solver.h:363-365,402-427.

// PropagateBoundsForTouchedRows preserves the source's deferred clearing:
// cancellation leaves every touched row marked, including completed rows.
func (s *LARSolver) PropagateBoundsForTouchedRows(bp *LARBoundPropagator) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if !s.Settings.UseTableau() {
		return nil
	}
	for _, row := range s.RowsWithChangedBounds.Indices() {
		if err = s.CalculateImpliedBoundsForRow(row, bp); err != nil {
			return err
		}
		cancel, e := s.Settings.CancelFlag()
		if e != nil {
			return e
		}
		if cancel {
			return nil
		}
	}
	if err = s.RowsWithChangedBounds.Clear(); err != nil {
		return err
	}
	// Settings remain live through the row hooks and cancellation provider.
	if !s.Settings.UseTableau() {
		return s.PropagateBoundsOnTerms(bp)
	}
	return nil
}

func (s *LARSolver) PropagateBoundsOnTerms(bp *LARBoundPropagator) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	for i, term := range s.Terms {
		used, e := s.TermIsUsedAsRow(LARTermStartIndex + uint32(i))
		if e != nil {
			return e
		}
		if used {
			continue
		}
		if err = s.propagateBoundsOnATerm(term, bp, uint32(i)); err != nil {
			return err
		}
	}
	return nil
}

func (s *LARSolver) propagateBoundsOnATerm(_ *LARTerm, _ *LARBoundPropagator, _ uint32) error {
	// This retained source body is SASSERT(false), with no native action.
	return sourceAssertionFailure()
}
