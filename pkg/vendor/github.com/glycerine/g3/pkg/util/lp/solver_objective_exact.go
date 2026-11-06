package lp

// Source: src/util/lp/lp_solver.hpp:95-100,528-577. Retained mpq/mpq
// objective preparation and the original-column reconstruction accessor.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalLPSolver) FlipCosts() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	for _, c := range m.Columns.All() {
		if c == nil {
			return ErrBounds
		}
		m.work = NewIndexedRationalVector(m.manager, 2)
		// The by-value getter precedes unary minus's own copied temporary.
		if err = c.GetCost(&m.work.Data[0]); err != nil {
			return err
		}
		if err = m.work.copySlot(&m.work.Data[1], &m.work.Data[0]); err != nil {
			return err
		}
		if err = m.presolveNegate(&m.work.Data[1]); err != nil {
			return err
		}
		if err = c.SetCost(&m.work.Data[1]); err != nil {
			return err
		}
		if err = m.finishPreparationWork(); err != nil {
			return err
		}
	}
	return nil
}

// SetScaledCost writes a preallocated dense Costs slot. The primal/dual outer
// preparation owns its extent; this method does not grow or clear that vector.
func (m *RationalLPSolver) SetScaledCost(j uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if m.ColumnScale == nil {
		return ErrBounds
	}
	if uint64(j) >= uint64(len(m.ColumnScale.Data)) {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	external, exists := m.CoreColumnsToExternal[j]
	if !exists {
		m.CoreColumnsToExternal[j] = 0
	}
	c, exists := m.Columns.Get2(external)
	if !exists {
		m.Columns.Set(external, nil)
	}
	if c == nil {
		return ErrBounds
	}
	m.work = NewIndexedRationalVector(m.manager, 3)
	if err = c.GetCost(&m.work.Data[0]); err != nil {
		return err
	}
	if c.IsFlipped() {
		minusOne := numeral.NewRational()
		if err = m.manager.SetInt64(&minusOne, -1); err != nil {
			return err
		}
		if err = m.presolveMultiply(&m.work.Data[0], &minusOne, &m.work.Data[0]); err != nil {
			return err
		}
	}
	if c.Fixed {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if uint64(j) >= uint64(len(m.ColumnScale.Data)) {
		return ErrBounds
	}
	if err = m.expressionProduct(&m.work.Data[1], &m.work.Data[2], &m.work.Data[0], &m.ColumnScale.Data[j]); err != nil {
		return err
	}
	if m.Costs == nil || uint64(j) >= uint64(len(m.Costs.Data)) {
		return ErrBounds
	}
	if err = m.Costs.copySlot(&m.Costs.Data[j], &m.work.Data[2]); err != nil {
		return err
	}
	return m.finishPreparationWork()
}

// ColumnValueFromCore specializes the source get_var_value accessor, whose
// implementation returns a const reference to X[j]. coreX is borrowed exact
// storage, not an optimization adapter. This does not implement a core solver.
// A caller-owned out must outlive Close after an interrupted numerical call.
func (m *RationalLPSolver) ColumnValueFromCore(column uint32, coreX []numeral.Rational, out *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	c, exists := m.Columns.Get2(column)
	if !exists {
		zero := numeral.NewRational()
		return m.copyValue(out, &zero)
	}
	if c == nil {
		return ErrBounds
	}
	if c.Fixed {
		return c.GetFixedValue(out)
	}
	j := c.Index
	if j == InvalidColumnIndex {
		zero := numeral.NewRational()
		return m.copyValue(out, &zero)
	}
	if uint64(j) >= uint64(len(coreX)) || m.ColumnScale == nil || uint64(j) >= uint64(len(m.ColumnScale.Data)) {
		return ErrBounds
	}
	m.work = NewIndexedRationalVector(m.manager, 6)
	w := m.work
	if err = m.expressionProduct(&w.Data[0], &w.Data[1], &coreX[j], &m.ColumnScale.Data[j]); err != nil {
		return err
	}
	value := &w.Data[1]
	if !c.IsFree() {
		if !c.IsFlipped() {
			if err = c.GetLowBound(&w.Data[2]); err != nil {
				return err
			}
		} else {
			if err = w.copySlot(&w.Data[3], value); err != nil {
				return err
			}
			if err = m.presolveNegate(&w.Data[3]); err != nil {
				return err
			}
			value = &w.Data[3]
			if err = c.GetUpperBound(&w.Data[2]); err != nil {
				return err
			}
		}
		if err = m.expressionSum(&w.Data[4], &w.Data[5], value, &w.Data[2]); err != nil {
			return err
		}
		value = &w.Data[5]
	}
	if err = m.copyValue(out, value); err != nil {
		return err
	}
	return m.finishPreparationWork()
}
