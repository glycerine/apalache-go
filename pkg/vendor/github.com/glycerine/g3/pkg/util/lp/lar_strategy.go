package lp

// Source: src/util/lp/init_lar_solver.h:229-263;
// src/util/lp/lar_solver.h:985-994. Retained double/double matrix copy.

func (s *LARSolver) DecideOnStrategyAndAdjustInitialState() (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if sourceDebugEnabled && !s.StrategyIsUndecided() {
		return ErrAssertion
	}
	if s.Bounds.Witnesses.Size() > s.Settings.ColumnThresholdForLUInLAR {
		s.Settings.SimplexStrategy = StrategyLU
	} else {
		s.Settings.SimplexStrategy = StrategyTableauRows
	}
	return s.AdjustInitialState()
}

func (s *LARSolver) AdjustInitialState() (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	switch s.Settings.SimplexStrategy {
	case StrategyLU:
		return s.AdjustInitialStateForLU()
	case StrategyTableauCosts:
		if sourceDebugEnabled {
			return ErrAssertion
		}
		fallthrough // Literal source switch falls through when SASSERT erases.
	case StrategyUndecided, StrategyTableauRows:
		return s.AdjustInitialStateForTableauRows()
	}
	return nil
}

func (s *LARSolver) AdjustInitialStateForLU() (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	c := s.Core
	if err = s.CopyPairMatrixToFloat(c.DA); err != nil {
		return err
	}
	n := int(c.DA.ColumnCount())
	c.DX = resizeFloatSolverValues(c.DX, n)
	c.DLow = resizeFloatSolverValues(c.DLow, n)
	c.DUpper = resizeFloatSolverValues(c.DUpper, n)
	c.DHeading = append(c.DHeading[:0], c.RHeading...)
	c.DBasis = append(c.DBasis[:0], c.RBasis...)
	// Source deliberately does not assign m_d_nbasis or initialize an LU here.
	return nil
}

// CopyPairMatrixToFloat resizes the directories, preserving existing row and
// column contents, then appends converted cells in source row order. It neither
// clears duplicate cells nor initializes the matrix's row-offset scratch.
func (s *LARSolver) CopyPairMatrixToFloat(a *FloatStaticMatrix) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if a == nil {
		return ErrBounds
	}
	rows, columns := int(s.Core.RA.RowCount()), int(s.Core.RA.ColumnCount())
	if len(a.Rows) > rows {
		a.Rows = a.Rows[:rows]
	} else {
		a.Rows = append(a.Rows, make([][]FloatRowCell, rows-len(a.Rows))...)
	}
	if len(a.Columns) > columns {
		a.Columns = a.Columns[:columns]
	} else {
		a.Columns = append(a.Columns, make([][]ColumnCell, columns-len(a.Columns))...)
	}
	for i := range a.Rows {
		for k := range s.Core.RA.Rows[i] {
			cell := &s.Core.RA.Rows[i][k]
			value, e := s.manager.Float64(&cell.Value)
			if e != nil {
				return e
			}
			if err = a.Set(uint32(i), cell.Column, value); err != nil {
				return err
			}
		}
	}
	return nil
}
