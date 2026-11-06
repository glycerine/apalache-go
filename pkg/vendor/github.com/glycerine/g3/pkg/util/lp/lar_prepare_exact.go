package lp

// Source: src/util/lp/lar_core_solver.hpp:199-222;
// lp_primal_core_solver.h:826-838.

func resizeLARCounts(values []uint32, size uint32) []uint32 {
	if uint64(len(values)) >= uint64(size) {
		return values[:size]
	}
	return append(values, make([]uint32, int(size)-len(values))...)
}

// InitColumnRowNonZeroes fills only zero estimates. Nonzero estimates from
// prior basis solves survive, even when they differ from stored matrix lengths.
func (m *PairPrimalCoreSolver) InitColumnRowNonZeroes() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	m.ColumnsNZ = resizeLARCounts(m.ColumnsNZ, m.A.ColumnCount())
	m.RowsNZ = resizeLARCounts(m.RowsNZ, m.A.RowCount())
	for j := range m.ColumnsNZ {
		if m.ColumnsNZ[j] == 0 {
			m.ColumnsNZ[j] = uint32(len(m.A.Columns[j]))
		}
	}
	for i := range m.RowsNZ {
		if m.RowsNZ[i] == 0 {
			m.RowsNZ[i] = uint32(len(m.A.Rows[i]))
		}
	}
	return nil
}

// PrefixR prepares the existing precise core without rebuilding the basis or
// initializing optimization. Ordinary source vectors retain their prefix;
// indexed source scratch is cleared before resizing.
func (c *LARCoreSolver) PrefixR() (err error) {
	if c.stopped {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	r := c.RPrimal
	if err = r.ready(); err != nil {
		return err
	}
	n, rows := c.RA.ColumnCount(), c.RA.RowCount()
	if !r.Settings.UseTableau() {
		if err = r.CopyXB.resizeData(n); err != nil {
			return err
		}
		if err = r.Ed.Resize(rows); err != nil {
			return err
		}
		if err = r.PivotRow.Resize(n); err != nil {
			return err
		}
		if err = r.PivotRowB1.Resize(rows); err != nil {
			return err
		}
		if err = r.W.Resize(rows); err != nil {
			return err
		}
		if err = r.Y.resizeData(rows); err != nil {
			return err
		}
		r.RowsNZ = resizeLARCounts(r.RowsNZ, rows)
		r.ColumnsNZ = resizeLARCounts(r.ColumnsNZ, n)
		if err = r.InitColumnRowNonZeroes(); err != nil {
			return err
		}
	}
	if err = c.RightSides.resizeData(rows); err != nil {
		return err
	}
	if r.Settings.SimplexStrategy != StrategyTableauRows {
		if r.Settings.UseBreakpointsInFeasibilitySearch {
			if err = r.BreakpointQueue.Resize(n); err != nil {
				return err
			}
		}
		if err = c.Costs.resizeData(n); err != nil {
			return err
		}
		if err = r.D.resizeData(n); err != nil {
			return err
		}
		r.UsingInfeasCosts = true
	}
	return nil
}
