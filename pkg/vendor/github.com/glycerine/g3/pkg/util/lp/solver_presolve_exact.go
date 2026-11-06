package lp

// Source: src/util/lp/lp_solver.hpp:168-411; lp_solver.h:187-193;
// lp_settings.h:350-356. Concrete retained lp_solver<mpq,mpq> presolve.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/insdict"
)

// Compound assignments mutate their existing left operand. Binary expressions
// instead use expressionProduct/expressionDifference and own the return copy.
func (m *RationalLPSolver) presolveMultiply(a, b, out *numeral.Rational) error {
	bef, err := m.calls.begin(m.manager)
	if err != nil {
		return err
	}
	err = m.manager.Mul(a, b, out)
	return m.calls.after(m.manager, bef, err)
}
func (m *RationalLPSolver) presolveAdd(a, b, out *numeral.Rational) error {
	bef, err := m.calls.begin(m.manager)
	if err != nil {
		return err
	}
	err = m.manager.Add(a, b, out)
	return m.calls.after(m.manager, bef, err)
}
func (m *RationalLPSolver) presolveSubtract(a, b, out *numeral.Rational) error {
	bef, err := m.calls.begin(m.manager)
	if err != nil {
		return err
	}
	err = m.manager.Sub(a, b, out)
	return m.calls.after(m.manager, bef, err)
}
func (m *RationalLPSolver) presolveNegate(v *numeral.Rational) error {
	bef, err := m.calls.begin(m.manager)
	if err != nil {
		return err
	}
	err = m.manager.Neg(v)
	return m.calls.after(m.manager, bef, err)
}

// Bounds are source out-refs: an unknown endpoint returns false with the
// prefix intact. Fixed flags do not replace bounds; out outlives Close on error.
func (m *RationalLPSolver) presolveRowBound(row *insdict.Dict[uint32, *numeral.Rational], out *numeral.Rational, maximum bool) (known bool, err error) {
	zero := numeral.NewRational()
	if err = m.copyValue(out, &zero); err != nil {
		return false, err
	}
	if row == nil {
		return false, ErrBounds
	}
	m.shiftWork = NewIndexedRationalVector(m.manager, 4)
	w := m.shiftWork
	defer func() {
		if err == nil {
			err = w.Close()
			if err == nil {
				m.shiftWork = nil
			}
		}
	}()
	for j, value := range row.All() {
		if value == nil {
			return false, ErrBounds
		}
		if err = w.copySlot(&w.Data[0], value); err != nil {
			return false, err
		}
		c, exists := m.Columns.Get2(j)
		if !exists {
			m.Columns.Set(j, nil)
		}
		if c == nil {
			return false, ErrBounds
		}
		var low bool
		if maximum {
			low, err = m.manager.IsNeg(&w.Data[0])
		} else {
			low, err = m.manager.IsPos(&w.Data[0])
		}
		if err != nil {
			return false, err
		}
		if low {
			if !c.LowSet {
				return false, nil
			}
			err = c.GetLowBound(&w.Data[1])
		} else {
			if !c.UpperSet {
				return false, nil
			}
			err = c.GetUpperBound(&w.Data[1])
		}
		if err != nil {
			return false, err
		}
		if err = m.expressionProduct(&w.Data[3], &w.Data[2], &w.Data[1], &w.Data[0]); err != nil {
			return false, err
		}
		if err = m.presolveAdd(out, &w.Data[2], out); err != nil {
			return false, err
		}
		for k := 2; k >= 0; k-- {
			if err = m.manager.Del(&w.Data[k]); err != nil {
				return false, err
			}
			w.Data[k] = numeral.NewRational()
		}
	}
	return true, nil
}
func (m *RationalLPSolver) MinimalRowValue(row *insdict.Dict[uint32, *numeral.Rational], out *numeral.Rational) (known bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	return m.presolveRowBound(row, out, false)
}
func (m *RationalLPSolver) MaximalRowValue(row *insdict.Dict[uint32, *numeral.Rational], out *numeral.Rational) (known bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	return m.presolveRowBound(row, out, true)
}

func (m *RationalLPSolver) pinVars(row *insdict.Dict[uint32, *numeral.Rational], sign *numeral.Rational) (err error) {
	if sign == nil {
		return ErrBounds
	}
	m.pinWork = NewIndexedRationalVector(m.manager, 6)
	w := m.pinWork
	defer func() {
		if err == nil {
			err = w.Close()
			if err == nil {
				m.pinWork = nil
			}
		}
	}()
	// T sign is by value, before the function body and its row loop.
	if err = w.copySlot(&w.Data[0], sign); err != nil {
		return err
	}
	if row == nil {
		return ErrBounds
	}
	for j, value := range row.All() {
		if value == nil {
			return ErrBounds
		}
		if err = w.copySlot(&w.Data[1], value); err != nil {
			return err
		} // auto t
		c, exists := m.Columns.Get2(j)
		if !exists {
			m.Columns.Set(j, nil)
		}
		if c == nil {
			return ErrBounds
		}
		if err = w.copySlot(&w.Data[2], &w.Data[1]); err != nil {
			return err
		} // T a
		if err = m.expressionProduct(&w.Data[5], &w.Data[3], &w.Data[2], &w.Data[0]); err != nil {
			return err
		}
		var upper bool
		upper, err = m.manager.IsPos(&w.Data[3])
		if err != nil {
			return err
		}
		// The condition's product dies before entering either branch.
		if err = m.manager.Del(&w.Data[3]); err != nil {
			return err
		}
		w.Data[3] = numeral.NewRational()
		if upper {
			err = c.GetUpperBound(&w.Data[4])
		} else {
			err = c.GetLowBound(&w.Data[4])
		}
		if err != nil {
			return err
		}
		if err = c.SetFixedValue(&w.Data[4]); err != nil {
			return err
		}
		for k := 4; k >= 1; k-- {
			if err = m.manager.Del(&w.Data[k]); err != nil {
				return err
			}
			w.Data[k] = numeral.NewRational()
		}
	}
	return nil
}
func (m *RationalLPSolver) PinVarsOnRowWithSign(row *insdict.Dict[uint32, *numeral.Rational], sign *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.pinVars(row, sign)
}

func (m *RationalLPSolver) presolveRowIsZero(row *insdict.Dict[uint32, *numeral.Rational]) (bool, error) {
	if row == nil {
		return false, ErrBounds
	}
	for _, v := range row.All() {
		if v == nil {
			return false, ErrBounds
		}
		zero, err := m.manager.IsZero(v)
		if err != nil || !zero {
			return false, err
		}
	}
	return true, nil
}
func (m *RationalLPSolver) defaultConstraint(index uint32) *RationalConstraint {
	if c, exists := m.Constraints.Get2(index); exists {
		return c
	}
	// mpq RHS default-constructs to zero, while the relation is indeterminate.
	c := &RationalConstraint{RightSide: m.newValue()}
	m.Constraints.Set(index, c)
	return c
}
func (m *RationalLPSolver) rowBoundObsolete(row *insdict.Dict[uint32, *numeral.Rational], maximum bool) (bool, error) {
	w := m.work
	bound := &w.Data[1]
	if maximum {
		bound = &w.Data[2]
	}
	known, err := m.presolveRowBound(row, bound, maximum)
	if err != nil || !known {
		return false, err
	}
	if maximum {
		err = m.expressionDifference(&w.Data[5], &w.Data[3], &w.Data[0], bound)
	} else {
		err = m.expressionDifference(&w.Data[5], &w.Data[3], bound, &w.Data[0])
	}
	if err != nil {
		return false, err
	}
	if m.Settings == nil {
		return false, ErrBounds
	}
	// precise val_is_smaller_than_eps(t,eps) is t<=0; the float argument
	// is ignored. Still construct the actual -diff temporary for its second call.
	positive, err := m.manager.IsPos(&w.Data[3])
	if err != nil {
		return false, err
	}
	if positive {
		m.Status = StatusInfeasible
		return true, nil
	}
	if err = w.copySlot(&w.Data[4], &w.Data[3]); err != nil {
		return false, err
	}
	if err = m.presolveNegate(&w.Data[4]); err != nil {
		return false, err
	}
	positive, err = m.manager.IsPos(&w.Data[4])
	if err != nil {
		return false, err
	}
	// The unary-minus condition temporary dies before pinning the row.
	if err = m.manager.Del(&w.Data[4]); err != nil {
		return false, err
	}
	w.Data[4] = numeral.NewRational()
	if !positive {
		sign := numeral.NewRational()
		n := int64(-1)
		if maximum {
			n = 1
		}
		if err = m.manager.SetInt64(&sign, n); err != nil {
			return false, err
		}
		if err = m.pinVars(row, &sign); err != nil {
			return false, err
		}
		return true, nil
	}
	for k := 4; k >= 3; k-- {
		if err = m.manager.Del(&w.Data[k]); err != nil {
			return false, err
		}
		w.Data[k] = numeral.NewRational()
	}
	return false, nil
}
func (m *RationalLPSolver) rowIsObsolete(row *insdict.Dict[uint32, *numeral.Rational], index uint32) (obsolete bool, err error) {
	c := m.defaultConstraint(index)
	if c == nil || !c.relationSet {
		return false, ErrBounds
	}
	switch c.Relation {
	case RelationEqual, RelationGreaterOrEqual, RelationLessOrEqual:
	default:
		return false, sourceAssertionFailure()
	}
	if c.RightSide == nil {
		return false, ErrBounds
	}
	m.work = NewIndexedRationalVector(m.manager, 6)
	defer func() {
		if err == nil {
			err = m.finishPreparationWork()
		}
	}()
	if err = m.work.copySlot(&m.work.Data[0], c.RightSide); err != nil {
		return false, err
	}
	zero, err := m.presolveRowIsZero(row)
	if err != nil {
		return false, err
	}
	if zero {
		var bad bool
		switch c.Relation {
		case RelationEqual:
			var z bool
			z, err = m.manager.IsZero(&m.work.Data[0])
			bad = !z
		case RelationGreaterOrEqual:
			bad, err = m.manager.IsPos(&m.work.Data[0])
		case RelationLessOrEqual:
			bad, err = m.manager.IsNeg(&m.work.Data[0])
		}
		if err != nil {
			return false, err
		}
		if bad {
			m.Status = StatusInfeasible
		}
		return true, nil
	}
	if c.Relation != RelationGreaterOrEqual {
		obsolete, err = m.rowBoundObsolete(row, false)
		if err != nil || obsolete {
			return obsolete, err
		}
	}
	if c.Relation != RelationLessOrEqual {
		return m.rowBoundObsolete(row, true)
	}
	return false, nil
}
func (m *RationalLPSolver) RowIsObsolete(row *insdict.Dict[uint32, *numeral.Rational], index uint32) (obsolete bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	return m.rowIsObsolete(row, index)
}

func (m *RationalLPSolver) releaseInputValue(v *numeral.Rational) error {
	if v == nil {
		return ErrBounds
	}
	if err := m.manager.Del(v); err != nil {
		return err
	}
	*v = numeral.NewRational()
	return nil
}
func (m *RationalLPSolver) removeFixedFromRow(index uint32, row *insdict.Dict[uint32, *numeral.Rational]) (err error) {
	c := m.defaultConstraint(index)
	if c == nil || c.RightSide == nil || row == nil {
		return ErrBounds
	}
	var removed []uint32
	for j, value := range row.All() {
		ci, exists := m.Columns.Get2(j)
		if !exists {
			if err = sourceAssertionFailure(); err != nil {
				return err
			}
			m.Columns.Set(j, nil)
		}
		if ci == nil || value == nil {
			return ErrBounds
		}
		if ci.Fixed {
			removed = append(removed, j)
			m.work = NewIndexedRationalVector(m.manager, 4)
			if err = m.work.copySlot(&m.work.Data[0], value); err != nil {
				return err
			}
			if err = ci.GetFixedValue(&m.work.Data[1]); err != nil {
				return err
			}
			if err = m.expressionProduct(&m.work.Data[3], &m.work.Data[2], &m.work.Data[0], &m.work.Data[1]); err != nil {
				return err
			}
			if err = m.presolveSubtract(c.RightSide, &m.work.Data[2], c.RightSide); err != nil {
				return err
			}
			if err = m.finishPreparationWork(); err != nil {
				return err
			}
		} else {
			var zero bool
			zero, err = m.manager.IsZero(value)
			if err != nil {
				return err
			}
			if zero {
				removed = append(removed, j)
			}
		}
	}
	for _, j := range removed {
		v, _ := row.Get2(j)
		if err = m.releaseInputValue(v); err != nil {
			return err
		}
		row.Del(j)
	}
	return nil
}
func (m *RationalLPSolver) RemoveFixedOrZeroColumnsFromRow(index uint32, row *insdict.Dict[uint32, *numeral.Rational]) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.removeFixedFromRow(index, row)
}
func (m *RationalLPSolver) removeFixedColumns() error {
	for i, row := range m.AValues.All() {
		if err := m.removeFixedFromRow(i, row); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalLPSolver) RemoveFixedOrZeroColumns() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.removeFixedColumns()
}
func (m *RationalLPSolver) tryToRemoveRows() (uint32, error) {
	var removed []uint32
	for i, row := range m.AValues.All() {
		obsolete, err := m.rowIsObsolete(row, i)
		if err != nil {
			return 0, err
		}
		if obsolete {
			removed = append(removed, i)
		}
		if m.Status == StatusInfeasible {
			return 0, nil
		}
	}
	for _, i := range removed {
		row, _ := m.AValues.Get2(i)
		for _, value := range row.All() {
			if err := m.releaseInputValue(value); err != nil {
				return 0, err
			}
		}
		m.AValues.Del(i)
	}
	if err := m.removeFixedColumns(); err != nil {
		return 0, err
	}
	return uint32(len(removed)), nil
}
func (m *RationalLPSolver) TryToRemoveSomeRows() (removed uint32, err error) {
	if err = m.ready(); err != nil {
		return 0, err
	}
	defer func() { m.stopped = err != nil }()
	return m.tryToRemoveRows()
}
func (m *RationalLPSolver) Cleanup() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	var passes int32
	for {
		n, e := m.tryToRemoveRows()
		if e != nil {
			return e
		}
		if n == 0 {
			break
		}
		passes++
	}
	if passes == 0 {
		return nil
	}
	if m.Settings == nil {
		return ErrBounds
	}
	out := m.Settings.DebugOut
	if out == nil {
		return nil
	}
	text := "deleted one row\n"
	if passes != 1 {
		text = fmt.Sprintf("deleted %d rows\n", passes)
	}
	if err = writeConstraintText(out, text); err != nil {
		return err
	}
	return flushConstraintOutput(out)
}
