package lp

// Source: src/util/lp/lar_core_solver.h:170-264, core scope lifecycle.

func pushLARUintVector(dst *StackedUintVector, src []uint32) error {
	if sourceDebugEnabled && uint64(dst.Size()) > uint64(len(src)) {
		return ErrAssertion
	}
	for i, value := range src {
		if uint32(i) == dst.Size() {
			dst.Append(value)
		} else if err := dst.Set(uint32(i), value); err != nil {
			return err
		}
	}
	dst.Push()
	return nil
}

// Push snapshots only the source core's fields. Outer witness/status,
// constraints, terms and external directories belong to LARSolver's scopes.
// Each numeric child owns its calls; no outer capture wraps scope operations.
func (c *LARCoreSolver) Push() (err error) {
	if c.stopped {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if err = c.RPrimal.ready(); err != nil {
		return err
	}
	if sourceDebugEnabled {
		ok, e := c.RPrimal.BasisHeadingIsCorrect()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
		if c.NeedToPresolveWithDoubleSolver() {
			ok, e = c.DPrimal.BasisHeadingIsCorrect()
			if e != nil {
				return e
			}
			if !ok {
				return ErrAssertion
			}
		}
		if c.Bounds.Types.Size() != c.RA.ColumnCount() {
			return ErrAssertion
		}
	}
	c.StackedSimplexStrategy.Value = c.settings.SimplexStrategy
	c.StackedSimplexStrategy.Push()
	c.Bounds.Types.Push()
	if !c.settings.UseTableau() {
		if err = c.RA.Push(); err != nil {
			return err
		}
	}
	if err = c.Bounds.Low.Push(); err != nil {
		return err
	}
	if err = c.Bounds.Upper.Push(); err != nil {
		return err
	}
	if !c.settings.UseTableau() {
		if err = pushLARUintVector(&c.RPushedBasis, c.RBasis); err != nil {
			return err
		}
		if err = pushLARUintVector(&c.RColumnsNZ, c.RPrimal.ColumnsNZ); err != nil {
			return err
		}
		if err = pushLARUintVector(&c.RRowsNZ, c.RPrimal.RowsNZ); err != nil {
			return err
		}
	}
	c.DA.Push()
	if !c.settings.UseTableau() {
		return pushLARUintVector(&c.DPushedBasis, c.DBasis)
	}
	return nil
}

func (c *LARCoreSolver) popMarkowitzCounts(k uint32) error {
	if err := c.RColumnsNZ.Pop(k); err != nil {
		return err
	}
	if err := c.RRowsNZ.Pop(k); err != nil {
		return err
	}
	// Source resize followed by element assignment; these are separate owners.
	c.RPrimal.ColumnsNZ = append(c.RPrimal.ColumnsNZ[:0], c.RColumnsNZ.Values()...)
	c.RPrimal.RowsNZ = append(c.RPrimal.RowsNZ[:0], c.RRowsNZ.Values()...)
	return nil
}

func (c *LARCoreSolver) popBasis(k uint32) error {
	if !c.settings.UseTableau() {
		if err := c.RPushedBasis.Pop(k); err != nil {
			return err
		}
		c.RBasis = append(c.RBasis[:0], c.RPushedBasis.Values()...)
		if err := c.RPrimal.InitBasisHeadingAndNonBasicColumns(); err != nil {
			return err
		}
		if err := c.DPushedBasis.Pop(k); err != nil {
			return err
		}
		c.DBasis = append(c.DBasis[:0], c.DPushedBasis.Values()...)
		return c.DPrimal.InitBasisHeadingAndNonBasicColumns()
	}
	c.DBasis = append(c.DBasis[:0], c.RBasis...)
	c.DNBasis = append(c.DNBasis[:0], c.RNBasis...)
	c.DHeading = append(c.DHeading[:0], c.RHeading...)
	return nil
}

// Pop restores the core in literal source order. Matrix scopes restore only
// dimensions, surviving X/cost/D values remain current, and both factorizations
// are destroyed. In tableau mode the outer solver must remove rational tableau
// columns first; this core deliberately neither snapshots nor pops that matrix.
func (c *LARCoreSolver) Pop(k uint32) (err error) {
	if c.stopped {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if err = c.RPrimal.ready(); err != nil {
		return err
	}
	if !c.settings.UseTableau() {
		if err = c.RA.Pop(k); err != nil {
			return err
		}
	}
	if err = c.Bounds.Low.Pop(k); err != nil {
		return err
	}
	if err = c.Bounds.Upper.Pop(k); err != nil {
		return err
	}
	if err = c.Bounds.Types.Pop(k); err != nil {
		return err
	}
	if c.RPrimal.Factorization != nil {
		if err = c.RPrimal.Factorization.Close(); err != nil {
			return err
		}
		c.RPrimal.Factorization = nil
	}
	if err = c.RX.resizeData(c.RA.ColumnCount()); err != nil {
		return err
	}
	if err = c.Costs.resizeData(c.RA.ColumnCount()); err != nil {
		return err
	}
	if err = c.RPrimal.D.resizeData(c.RA.ColumnCount()); err != nil {
		return err
	}
	if !c.settings.UseTableau() {
		if err = c.popMarkowitzCounts(k); err != nil {
			return err
		}
	}
	if err = c.DA.Pop(k); err != nil {
		return err
	}
	if c.DPrimal.Factorization != nil {
		if err = c.DPrimal.Factorization.Close(); err != nil {
			return err
		}
		c.DPrimal.Factorization = nil
	}
	c.DX = resizeFloatSolverValues(c.DX, int(c.DA.ColumnCount()))
	if err = c.popBasis(k); err != nil {
		return err
	}
	c.StackedSimplexStrategy.Pop(k)
	c.settings.SimplexStrategy = c.StackedSimplexStrategy.Value
	if sourceDebugEnabled {
		ok, e := c.RPrimal.BasisHeadingIsCorrect()
		if e != nil {
			return e
		}
		if !ok {
			return ErrAssertion
		}
		if c.NeedToPresolveWithDoubleSolver() {
			ok, e = c.DPrimal.BasisHeadingIsCorrect()
			if e != nil {
				return e
			}
			if !ok {
				return ErrAssertion
			}
		}
	}
	return nil
}
