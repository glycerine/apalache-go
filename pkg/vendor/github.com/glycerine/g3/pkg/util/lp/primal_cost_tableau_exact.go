package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:1121-1163;
// lp_primal_core_solver_tableau.h:385-395; lp_primal_core_solver.h:881-891.

import "github.com/glycerine/g3/pkg/util/numeral"

// GetInfeasibilityCostForColumn writes the exact source cost. Nonbasic columns
// return zero before reading types or bounds; boxed/fixed columns test upper first.
// The caller owns out, including any assignment prefix on failure.
func (m *PairPrimalCoreSolver) GetInfeasibilityCostForColumn(j uint32, out *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if out == nil || uint64(j) >= uint64(len(*m.Heading)) {
		return ErrBounds
	}
	var cost int32
	if (*m.Heading)[j] >= 0 {
		if uint64(j) >= uint64(len(*m.ColumnTypes)) {
			return ErrBounds
		}
		kind := (*m.ColumnTypes)[j]
		if kind == ColumnFixed || kind == ColumnBoxed || kind == ColumnUpperBound || kind == ColumnLowBound {
			if uint64(j) >= uint64(len(*m.X)) {
				return ErrBounds
			}
			x := &(*m.X)[j]
			above := false
			if kind == ColumnFixed || kind == ColumnBoxed || kind == ColumnUpperBound {
				if uint64(j) >= uint64(len(*m.UpperBounds)) {
					return ErrBounds
				}
				above, err = m.AboveBound(x, &(*m.UpperBounds)[j])
				if err != nil {
					return err
				}
			}
			if above {
				cost = 1
			} else if kind == ColumnFixed || kind == ColumnBoxed || kind == ColumnLowBound {
				if uint64(j) >= uint64(len(*m.LowBounds)) {
					return ErrBounds
				}
				below, e := m.BelowBound(x, &(*m.LowBounds)[j])
				if e != nil {
					return e
				}
				if below {
					cost = -1
				}
			}
		} else if kind != ColumnFree {
			if err = sourceAssertionFailure(); err != nil {
				return err
			}
		}
		if !m.Settings.UseBreakpointsInFeasibilitySearch {
			cost = -cost
		}
	}
	// Bound comparisons belong to the base core; only this assignment belongs
	// to the primal invocation. Never capture across a delegated numeric owner.
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	err = m.manager.SetInt32(out, cost)
	return m.calls.after(m.manager, before, err)
}

// updateReducedCostForBasicColumnCostChange uses the caller's retained work.
// delta is old cost minus new cost. Stored row order and delta aliases follow
// the source, including a completed prefix when a later coefficient fails.
func (m *PairPrimalCoreSolver) updateReducedCostForBasicColumnCostChange(delta *numeral.Rational, j uint32) error {
	if delta == nil || uint64(j) >= uint64(len(*m.Heading)) {
		return ErrBounds
	}
	row := (*m.Heading)[j]
	if sourceDebugEnabled && row < 0 {
		return ErrAssertion
	}
	if row < 0 || uint64(row) >= uint64(len(m.A.Rows)) {
		return ErrBounds
	}
	for k := range m.A.Rows[row] {
		cell := &m.A.Rows[row][k]
		if cell.Column == j {
			continue
		}
		if uint64(cell.Column) >= uint64(len(m.D.Data)) {
			return ErrBounds
		}
		tmp, product := &m.work.Data[2], &m.work.Data[3]
		if err := etaRationalProduct(m.manager, tmp, product, delta, &cell.Value); err != nil {
			return err
		}
		dst := &m.D.Data[cell.Column]
		if err := m.manager.Add(dst, product, dst); err != nil {
			return err
		}
		if err := m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
	}
	return nil
}

func (m *PairPrimalCoreSolver) UpdateReducedCostForBasicColumnCostChange(delta *numeral.Rational, j uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	m.work = NewIndexedRationalVector(m.manager, 4)
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	err = m.updateReducedCostForBasicColumnCostChange(delta, j)
	if err = m.calls.after(m.manager, before, err); err != nil {
		return err
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}

func (m *PairPrimalCoreSolver) UpdateInfCostForColumnTableau(j uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled && (m.Settings.SimplexStrategy == StrategyTableauRows || !m.UsingInfeasCosts) {
		return ErrAssertion
	}
	// Get the source return value before installing the expression owner, so
	// its base comparisons retain their own failure receipts.
	newCost := numeral.NewRational()
	if err = m.GetInfeasibilityCostForColumn(j, &newCost); err != nil {
		return err
	}
	m.work = NewIndexedRationalVector(m.manager, 4)
	m.work.Data[0] = newCost
	if uint64(j) >= uint64(len(*m.Costs)) {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	err = func() error {
		tmp, delta := &m.work.Data[2], &m.work.Data[1]
		if e := etaRationalDifference(m.manager, tmp, delta, &(*m.Costs)[j], &m.work.Data[0]); e != nil {
			return e
		}
		zero, e := m.manager.IsZero(delta)
		if e != nil {
			return e
		}
		if zero {
			return nil
		}
		if e = m.manager.Copy(&(*m.Costs)[j], &m.work.Data[0]); e != nil {
			return e
		}
		return m.updateReducedCostForBasicColumnCostChange(delta, j)
	}()
	if err = m.calls.after(m.manager, before, err); err != nil {
		return err
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}
