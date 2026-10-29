package lp

// Source: src/util/lp/lp_solver.hpp:412-534, lp_solver.h:229-235 and precise
// scale early return at lp_solver.hpp:111-116; retained mpq/mpq consumers.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalLPSolver) mapExternalRows() {
	var next uint32
	for row := range m.AValues.All() {
		m.ExternalRowsToCore[row] = next
		m.CoreRowsToExternal[next] = row
		next++
	}
}
func (m *RationalLPSolver) MapExternalRowsToCoreSolverRows() error {
	if err := m.ready(); err != nil {
		return err
	}
	m.mapExternalRows()
	return nil
}
func (m *RationalLPSolver) mapExternalColumns() error {
	var next uint32
	for _, row := range m.AValues.All() {
		if row == nil {
			return ErrBounds
		}
		for j, value := range row.All() {
			if value == nil {
				return ErrBounds
			}
			zero, err := m.manager.IsZero(value)
			if err != nil {
				return err
			}
			if zero {
				return ErrLPFixedColumn
			}
			c, exists := m.Columns.Get2(j)
			if !exists {
				m.Columns.Set(j, nil)
			}
			if c == nil {
				return ErrBounds
			}
			if c.Fixed {
				return ErrLPFixedColumn
			}
			if !IsValidColumnIndex(c.Index) {
				c.Index = next
				m.CoreColumnsToExternal[next] = j
				next++
			}
		}
	}
	return nil
}
func (m *RationalLPSolver) MapExternalColumnsToCoreSolverColumns() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.mapExternalColumns()
}
func (m *RationalLPSolver) finishPreparationWork() error {
	if err := m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}
func (m *RationalLPSolver) fillA() error {
	// The source overwrites its pointer on a repeated native entry. Retain
	// displaced owners until Close rather than leaking their exact numerals.
	if m.A != nil {
		m.previousMatrices = append(m.previousMatrices, m.A)
	}
	m.A = NewRationalStaticMatrix(m.manager, uint32(m.AValues.Len()), uint32(len(m.CoreColumnsToExternal)))
	for external, row := range m.AValues.All() {
		i, exists := m.ExternalRowsToCore[external]
		if err := lpSolverDomain(exists); err != nil {
			return err
		}
		if row == nil {
			return ErrBounds
		}
		for j, value := range row.All() {
			// Source `auto k` copies the coefficient before column lookup.
			m.work = NewIndexedRationalVector(m.manager, 2)
			if value == nil {
				return ErrBounds
			}
			if err := m.work.copySlot(&m.work.Data[0], value); err != nil {
				return err
			}
			c, exists := m.Columns.Get2(j)
			if err := lpSolverDomain(exists && c != nil); err != nil {
				return err
			}
			if err := lpSolverDomain(IsValidColumnIndex(c.Index)); err != nil {
				return err
			}
			v := &m.work.Data[0]
			if c.IsFlipped() {
				if err := m.work.copySlot(&m.work.Data[1], v); err != nil {
					return err
				}
				v = &m.work.Data[1]
				b, err := m.calls.begin(m.manager)
				if err != nil {
					return err
				}
				err = m.manager.Neg(v)
				if err = m.calls.after(m.manager, b, err); err != nil {
					return err
				}
			}
			if err := m.A.Set(i, c.Index, v); err != nil {
				return err
			}
			if err := m.finishPreparationWork(); err != nil {
				return err
			}
		}
	}
	return nil
}
func (m *RationalLPSolver) FillAFromAValues() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.fillA()
}
func (m *RationalLPSolver) FillMatrixAAndInitRightSide() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	m.mapExternalRows()
	if err = m.mapExternalColumns(); err != nil {
		return err
	}
	if m.A != nil {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if err = m.fillA(); err != nil {
		return err
	}
	// B is dense storage: resizing retains existing values without an indexed
	// vector Clear, and destroys a removed suffix before shortening its owner.
	return m.B.resizeData(m.A.RowCount())
}

// LowBoundShiftForRow returns the source sum in out. An interrupted numerical
// call can borrow out until this model closes; the caller keeps it alive.
func (m *RationalLPSolver) LowBoundShiftForRow(row uint32, out *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.lowBoundShift(row, out)
}
func (m *RationalLPSolver) lowBoundShift(row uint32, out *numeral.Rational) error {
	zero := numeral.NewRational()
	if err := m.copyValue(out, &zero); err != nil {
		return err
	}
	r, exists := m.AValues.Get2(row)
	if !exists {
		return ErrLPSolverMissingRow
	}
	if r == nil {
		return ErrBounds
	}
	m.shiftWork = NewIndexedRationalVector(m.manager, 4)
	w := m.shiftWork
	for j, value := range r.All() {
		if value == nil {
			return ErrBounds
		}
		if err := w.copySlot(&w.Data[0], value); err != nil {
			return err
		}
		c, exists := m.Columns.Get2(j)
		if !exists {
			m.Columns.Set(j, nil)
		}
		if c == nil {
			return ErrBounds
		}
		if err := c.Shift(&w.Data[1]); err != nil {
			return err
		}
		b, err := m.calls.begin(m.manager)
		if err != nil {
			return err
		}
		err = etaRationalProduct(m.manager, &w.Data[3], &w.Data[2], &w.Data[0], &w.Data[1])
		if err = m.calls.after(m.manager, b, err); err != nil {
			return err
		}
		b, err = m.calls.begin(m.manager)
		if err != nil {
			return err
		}
		err = m.manager.Add(out, &w.Data[2], out)
		if err = m.calls.after(m.manager, b, err); err != nil {
			return err
		}
		// Product, shift return and loop copy die at this full expression/loop
		// boundary, while the accumulated return value remains with the caller.
		for k := 2; k >= 0; k-- {
			if err := m.manager.Del(&w.Data[k]); err != nil {
				return err
			}
			w.Data[k] = numeral.NewRational()
		}
	}
	if err := w.Close(); err != nil {
		return err
	}
	m.shiftWork = nil
	return nil
}
func (m *RationalLPSolver) fillB() error {
	if m.A == nil {
		return ErrBounds
	}
	for i := m.A.RowCount(); i > 0; {
		i--
		ext, exists := m.CoreRowsToExternal[i]
		if !exists {
			m.CoreRowsToExternal[i] = 0
		}
		c, exists := m.Constraints.Get2(ext)
		if err := lpSolverDomain(exists && c != nil && c.RightSide != nil); err != nil {
			return err
		}
		m.work = NewIndexedRationalVector(m.manager, 3)
		if err := m.lowBoundShift(ext, &m.work.Data[0]); err != nil {
			return err
		}
		b, err := m.calls.begin(m.manager)
		if err != nil {
			return err
		}
		err = etaRationalDifference(m.manager, &m.work.Data[2], &m.work.Data[1], c.RightSide, &m.work.Data[0])
		if err = m.calls.after(m.manager, b, err); err != nil {
			return err
		}
		if uint64(i) >= uint64(len(m.B.Data)) {
			return ErrBounds
		}
		if err := m.B.copySlot(&m.B.Data[i], &m.work.Data[1]); err != nil {
			return err
		}
		if err := m.finishPreparationWork(); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalLPSolver) FillB() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.fillB()
}
func (m *RationalLPSolver) restoreColumnScales() error {
	one := numeral.NewRational()
	if err := m.manager.SetInt64(&one, 1); err != nil {
		return err
	}
	for i := range m.ColumnScale.Data {
		if err := m.ColumnScale.copySlot(&m.ColumnScale.Data[i], &one); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalLPSolver) RestoreColumnScalesToOne() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.restoreColumnScales()
}
func (m *RationalLPSolver) Scale() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	// precise() short-circuits before source settings.use_scaling is read.
	if err = m.ColumnScale.resizeData(0); err != nil {
		return err
	}
	if m.A == nil {
		return ErrBounds
	}
	if err = m.ColumnScale.resizeData(m.A.ColumnCount()); err != nil {
		return err
	}
	return m.restoreColumnScales()
}
func (m *RationalLPSolver) Unscale() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if m.A != nil {
		if err = m.A.Close(); err != nil {
			return err
		}
	}
	m.A = nil
	if err = m.fillA(); err != nil {
		return err
	}
	if err = m.restoreColumnScales(); err != nil {
		return err
	}
	return m.fillB()
}
func (m *RationalLPSolver) countSlacksForRow(i uint32) error {
	ext, exists := m.CoreRowsToExternal[i]
	if !exists {
		m.CoreRowsToExternal[i] = 0
	}
	c, exists := m.Constraints.Get2(ext)
	if err := lpSolverDomain(exists && c != nil); err != nil {
		return err
	}
	if !c.relationSet {
		return ErrBounds
	}
	switch c.Relation {
	case RelationEqual:
		m.Artificials++
	case RelationGreaterOrEqual, RelationLessOrEqual:
		m.Slacks++
		if uint64(i) >= uint64(len(m.B.Data)) {
			return ErrBounds
		}
		var positive bool
		var err error
		if c.Relation == RelationLessOrEqual {
			positive, err = m.manager.IsNeg(&m.B.Data[i])
		} else {
			positive, err = m.manager.IsPos(&m.B.Data[i])
		}
		if err != nil {
			return err
		}
		if positive {
			m.Artificials++
		}
	}
	return nil
}
func (m *RationalLPSolver) CountSlacksAndArtificialsForRow(i uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.countSlacksForRow(i)
}
func (m *RationalLPSolver) CountSlacksAndArtificials() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if m.A == nil {
		return ErrBounds
	}
	for i := m.A.RowCount(); i > 0; {
		i--
		if err = m.countSlacksForRow(i); err != nil {
			return err
		}
	}
	return nil
}
