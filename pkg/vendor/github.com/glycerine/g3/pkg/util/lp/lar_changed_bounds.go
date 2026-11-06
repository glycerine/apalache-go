package lp

// Source: src/util/lp/lar_solver.h:710-751,798-849.

// DetectRowsOfBoundChangeColumnForNBasicColumn solves the actual basis column.
// The caller prepares the row-set domain; existing marks are retained.
func (s *LARSolver) DetectRowsOfBoundChangeColumnForNBasicColumn(j uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if s.ColumnBuffer.DataSize() != s.Core.RA.RowCount() {
		err = s.ColumnBuffer.Resize(s.Core.RA.RowCount())
	} else {
		err = s.ColumnBuffer.Clear()
	}
	if err != nil {
		return err
	}
	if sourceDebugEnabled && (len(s.ColumnBuffer.Index) != 0 || !s.ColumnBuffer.IsOK()) {
		return ErrAssertion
	}
	if err = s.Core.RPrimal.SolveBdIndexed(j, s.ColumnBuffer); err != nil {
		return err
	}
	for _, row := range s.ColumnBuffer.Index {
		if err = s.RowsWithChangedBounds.Insert(row); err != nil {
			return err
		}
	}
	return nil
}

func (s *LARSolver) DetectRowsOfBoundChangeColumnForNBasicColumnTableau(j uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if uint64(j) >= uint64(len(s.Core.RA.Columns)) {
		return ErrBounds
	}
	for _, cell := range s.Core.RA.Columns[j] {
		if err = s.RowsWithChangedBounds.Insert(cell.Row); err != nil {
			return err
		}
	}
	return nil
}

func (s *LARSolver) DetectRowsWithChangedBoundsForColumn(j uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if uint64(j) >= uint64(len(s.Core.RHeading)) {
		return ErrBounds
	}
	if row := s.Core.RHeading[j]; row >= 0 {
		return s.RowsWithChangedBounds.Insert(uint32(row))
	}
	if s.Settings.UseTableau() {
		return s.DetectRowsOfBoundChangeColumnForNBasicColumnTableau(j)
	}
	return s.DetectRowsOfBoundChangeColumnForNBasicColumn(j)
}

// DetectRowsOfColumnWithBoundChange is the source's second identical entry.
func (s *LARSolver) DetectRowsOfColumnWithBoundChange(j uint32) error {
	return s.DetectRowsWithChangedBoundsForColumn(j)
}

func (s *LARSolver) DetectRowsWithChangedBounds() (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	for _, j := range s.Bounds.ChangedColumns.Indices() {
		if err = s.DetectRowsWithChangedBoundsForColumn(j); err != nil {
			return err
		}
	}
	return nil
}

func (s *LARSolver) UpdateXAndInfCostsForColumnWithChangedBounds(j uint32) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	c := s.Core
	if uint64(j) >= uint64(len(c.RHeading)) {
		return ErrBounds
	}
	if c.RHeading[j] >= 0 {
		if s.Settings.SimplexStrategy != StrategyTableauRows {
			was := c.RPrimal.InfSet.Contains(j)
			if err = c.RPrimal.UpdateColumnInInfSet(j); err != nil {
				return err
			}
			if was != c.RPrimal.InfSet.Contains(j) {
				return s.BasicColumnsWithChangedCost.Insert(j)
			}
			return nil
		}
		return c.RPrimal.UpdateColumnInInfSet(j)
	}
	value := NewRationalPair()
	delta := &value
	// Retain this local owner only on failure, after the numeric children have
	// finished recording their own unfinished operations.
	defer func() {
		if delta != nil {
			s.pendingCleanupDelta = delta
		}
	}()
	changed, e := c.RPrimal.MakeColumnFeasible(j, delta)
	if e != nil {
		return e
	}
	if changed {
		if err = s.ChangeBasicXByDeltaOnColumn(j, delta); err != nil {
			return err
		}
	}
	if err = delta.Del(s.manager); err != nil {
		return err
	}
	delta = nil
	return nil
}

func (s *LARSolver) UpdateXAndInfCostsForColumnsWithChangedBounds() (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	for _, j := range s.Bounds.ChangedColumns.Indices() {
		if err = s.UpdateXAndInfCostsForColumnWithChangedBounds(j); err != nil {
			return err
		}
	}
	return nil
}

func (s *LARSolver) UpdateXAndInfCostsForColumnsWithChangedBoundsTableau() (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if sourceDebugEnabled {
		correct, e := s.AxIsCorrect()
		if e != nil {
			return e
		}
		if !correct {
			return ErrAssertion
		}
	}
	if err = s.UpdateXAndInfCostsForColumnsWithChangedBounds(); err != nil {
		return err
	}
	if s.Settings.SimplexStrategy == StrategyTableauCosts {
		for _, j := range s.BasicColumnsWithChangedCost.Indices() {
			if err = s.Core.RPrimal.UpdateInfCostForColumnTableau(j); err != nil {
				return err
			}
		}
		if sourceDebugEnabled {
			correct, e := s.Core.RPrimal.ReducedCostsAreCorrectTableau()
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
