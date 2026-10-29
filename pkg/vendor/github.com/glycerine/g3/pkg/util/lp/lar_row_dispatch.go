package lp

// Source: src/util/lp/lar_solver.h:219-249. Exact row-analysis dispatch.

// AnalyzeNewBoundsOnRowTableau analyzes the actual tableau row. The source
// length cutoff precedes the tableau-mode assertion and analyzer construction.
// The propagator is borrowed; it owns every bound already published on error.
func (s *LARSolver) AnalyzeNewBoundsOnRowTableau(row uint32, bp *LARBoundPropagator) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if uint64(row) >= uint64(len(s.Core.RA.Rows)) {
		return ErrBounds
	}
	if uint64(len(s.Core.RA.Rows[row])) > uint64(s.Settings.MaxRowLengthForBoundPropagation) {
		return nil
	}
	s.pendingRowIterator, err = NewRationalRowIterator(s.Core.RA, row)
	if err != nil {
		return err
	}
	if sourceDebugEnabled && !s.Settings.UseTableau() {
		return ErrAssertion
	}
	return s.analyzePendingRow(row, bp)
}

// AnalyzeNewBoundsOnRow consumes the core's already prepared pivot row and
// prepends its unit basis coefficient, as iterator_on_pivot_row does. Computing
// that row and substituting basis variables in terms are separate source steps;
// this entry point does neither implicitly.
func (s *LARSolver) AnalyzeNewBoundsOnRow(row uint32, bp *LARBoundPropagator) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if sourceDebugEnabled && s.Settings.UseTableau() {
		return ErrAssertion
	}
	if uint64(row) >= uint64(len(s.Core.RBasis)) {
		return ErrBounds
	}
	s.pendingRowIterator = NewRationalPivotRowIterator(s.Core.RPrimal.PivotRow, s.Core.RBasis[row])
	return s.analyzePendingRow(row, bp)
}

func (s *LARSolver) analyzePendingRow(row uint32, bp *LARBoundPropagator) (err error) {
	if bp == nil {
		return ErrBounds
	}
	// No outer numeric capture: iterator, analyzer and propagator each own
	// their physical calls, including separately stopped virtual providers.
	rhs := NewRationalPair()
	s.pendingRowAnalyzer, err = AnalyzeLARRow(s.pendingRowIterator, &rhs, row, bp)
	if err != nil {
		return err
	}
	return s.closeRowAnalysis()
}

func (s *LARSolver) closeRowAnalysis() error {
	if s.pendingRowAnalyzer != nil {
		if err := s.pendingRowAnalyzer.Close(); err != nil {
			return err
		}
		s.pendingRowAnalyzer = nil
	}
	if s.pendingRowIterator != nil {
		if err := s.pendingRowIterator.Close(); err != nil {
			return err
		}
		s.pendingRowIterator = nil
	}
	return nil
}
