package lp

// Source: src/util/lp/lar_core_solver.hpp:240-272;
// lp_primal_core_solver.h:156-168. Retained mpq/impq explanation consumer.

func (m *PairPrimalCoreSolver) InfSignOfColumn(j uint32) (sign int32, err error) {
	if err = m.ready(); err != nil {
		return 0, err
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled {
		feasible, e := m.ColumnIsFeasible(j)
		if e != nil {
			return 0, e
		}
		if feasible {
			return 0, ErrAssertion
		}
	}
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return 0, ErrBounds
	}
	switch (*m.ColumnTypes)[j] {
	case ColumnFree:
		return 0, nil
	case ColumnLowBound:
		return 1, nil
	case ColumnFixed, ColumnBoxed:
		if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.UpperBounds)) {
			return 0, ErrBounds
		}
		above, e := m.AboveBound(&(*m.X)[j], &(*m.UpperBounds)[j])
		if e != nil {
			return 0, e
		}
		if above {
			return -1, nil
		}
		return 1, nil
	default:
		return -1, nil
	}
}

func (c *LARCoreSolver) FillNotImprovableZeroSumFromInfRow() (err error) {
	if c.stopped || c.evidenceNegated != nil {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	m := c.RPrimal
	if err = m.ready(); err != nil {
		return err
	}
	if sourceDebugEnabled {
		off, e := m.AMultXIsOff()
		if e != nil {
			return e
		}
		if off {
			return ErrAssertion
		}
	}
	i := m.InfRowIndexForTableau
	if uint64(i) >= uint64(len(c.RBasis)) {
		return ErrBounds
	}
	sign, e := m.InfSignOfColumn(c.RBasis[i])
	if e != nil {
		return e
	}
	c.InfeasibleSumSign = sign
	if err = c.Infeasibility.ready(); err != nil {
		return err
	}
	if err = c.Infeasibility.Close(); err != nil {
		return err
	}
	// Source clears the previous explanation before indexing this row.
	if uint64(i) >= uint64(len(c.RA.Rows)) {
		return ErrBounds
	}
	for k := range c.RA.Rows[i] {
		cell := &c.RA.Rows[i][k]
		if err = c.Infeasibility.appendTermCoefficient(cell.Column, &cell.Value); err != nil {
			return err
		}
	}
	return nil
}

func (c *LARCoreSolver) FillNotImprovableZeroSum() (err error) {
	if c.stopped || c.evidenceNegated != nil {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if c.settings.SimplexStrategy == StrategyTableauRows {
		return c.FillNotImprovableZeroSumFromInfRow()
	}
	m := c.RPrimal
	if err = m.ready(); err != nil {
		return err
	}
	c.InfeasibleSumSign = 1
	if c.settings.UseBreakpointsInFeasibilitySearch {
		c.InfeasibleSumSign = -1
	}
	if err = c.Infeasibility.ready(); err != nil {
		return err
	}
	if err = c.Infeasibility.Close(); err != nil {
		return err
	}
	for _, j := range *m.Basis {
		if uint64(j) >= uint64(len(*m.Costs)) {
			return ErrBounds
		}
		value := &(*m.Costs)[j]
		zero, e := c.manager.IsZero(value)
		if e != nil {
			return e
		}
		if !zero {
			if err = c.Infeasibility.appendTermCoefficient(j, value); err != nil {
				return err
			}
		}
	}
	for j := uint32(0); j < m.A.ColumnCount(); j++ {
		if uint64(j) >= uint64(len(*m.Heading)) {
			return ErrBounds
		}
		if (*m.Heading)[j] >= 0 {
			continue
		}
		if uint64(j) >= uint64(len(m.D.Data)) {
			return ErrBounds
		}
		value := &m.D.Data[j]
		zero, e := c.manager.IsZero(value)
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		// Unary minus constructs a copied rational before make_pair and the
		// vector's copied pair. Keep it alive through a stopped append.
		c.evidenceNegated = NewIndexedRationalVector(c.manager, 1)
		w := c.evidenceNegated
		if err = w.copySlot(&w.Data[0], value); err != nil {
			return err
		}
		before, e := w.calls.begin(c.manager)
		if e != nil {
			return e
		}
		e = c.manager.Neg(&w.Data[0])
		if err = w.calls.after(c.manager, before, e); err != nil {
			return err
		}
		if err = c.Infeasibility.appendTermCoefficient(j, &w.Data[0]); err != nil {
			return err
		}
		if err = w.Close(); err != nil {
			return err
		}
		c.evidenceNegated = nil
	}
	return nil
}
