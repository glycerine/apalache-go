package lp

// Source: src/util/lp/lar_core_solver.hpp:224-241.

// PrefixD prepares the existing floating presolver. Ordinary vectors retain
// values; indexed scratch, column norms and infeasibility membership are reset.
func (c *LARCoreSolver) PrefixD() (err error) {
	if c.stopped {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	r := c.DPrimal
	n, rows := int(c.DA.ColumnCount()), int(c.DA.RowCount())
	c.DRightSides = resizeFloatSolverValues(c.DRightSides, rows)
	r.BreakpointQueue.Resize(uint32(n))
	r.CopyXB = resizeFloatSolverValues(r.CopyXB, n)
	c.DCosts = resizeFloatSolverValues(c.DCosts, n)
	r.D = resizeFloatSolverValues(r.D, n)
	if err = r.Ed.Resize(uint32(rows)); err != nil {
		return err
	}
	if err = r.PivotRow.Resize(uint32(n)); err != nil {
		return err
	}
	if err = r.PivotRowB1.Resize(uint32(rows)); err != nil {
		return err
	}
	if err = r.W.Resize(uint32(rows)); err != nil {
		return err
	}
	r.Y = resizeFloatSolverValues(r.Y, rows)
	r.SteepestEdge = resizeFloatSolverValues(r.SteepestEdge, n)
	r.ColumnNorms = resizeFloatSolverValues(r.ColumnNorms[:0], n)
	for j := range r.ColumnNorms {
		r.ColumnNorms[j] = 2
	}
	if err = r.InfSet.Clear(); err != nil {
		return err
	}
	return r.InfSet.Resize(uint32(n))
}
