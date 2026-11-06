package lp

// Source: src/util/lp/lar_core_solver.h:585-601.

// RBasisIsOK is the retained debug-only tableau basis validator.
func (c *LARCoreSolver) RBasisIsOK() (bool, error) {
	if !sourceDebugEnabled || !c.settings.UseTableau() {
		return true, nil
	}
	for _, j := range c.RBasis {
		if uint64(j) >= uint64(len(c.RA.Columns)) || len(c.RA.Columns[j]) != 1 {
			return false, ErrAssertion
		}
		value, e := c.RA.ColumnValue(c.RA.Columns[j][0])
		if e != nil {
			return false, e
		}
		one, e := c.manager.IsOne(value)
		if e != nil {
			return false, e
		}
		if !one {
			return false, ErrAssertion
		}
	}
	for j, h := range c.RHeading {
		if h >= 0 {
			continue
		}
		if uint64(j) >= uint64(c.Bounds.Types.Size()) {
			return false, ErrBounds
		}
		kind, e := c.Bounds.Types.At(uint32(j))
		if e != nil {
			return false, e
		}
		if kind == ColumnFixed {
			continue
		}
		if h == -1<<31 {
			return false, ErrBounds
		}
		if uint64(-h-1) >= uint64(c.Bounds.Types.Size()) || h > -1 {
			return false, ErrAssertion
		}
	}
	return true, nil
}
