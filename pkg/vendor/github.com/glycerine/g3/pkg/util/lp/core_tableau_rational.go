package lp

// Source: src/util/lp/lp_core_solver_base.hpp:100-112,604-668; mpq/mpq.

import "github.com/glycerine/g3/pkg/util/numeral"

// DivideRowByPivot normalizes the RHS first, then nonpivot coefficients in
// stored row order, and only finally replaces the pivot with one.
func (m *RationalCoreSolverBase) DivideRowByPivot(row, column uint32) (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.A.ready(); err != nil {
		return false, err
	}
	if uint64(row) >= uint64(len(m.A.Rows)) {
		return false, ErrBounds
	}
	cells := m.A.Rows[row]
	pivot := -1
	for k := range cells {
		if cells[k].Column == column {
			pivot = k
			break
		}
	}
	if pivot < 0 {
		return false, nil
	}
	p := &cells[pivot].Value
	zero, e := m.manager.IsZero(p)
	if e != nil {
		return false, e
	}
	if zero {
		return false, nil
	}
	if uint64(row) >= uint64(len(*m.B)) {
		return false, ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	rhs := &(*m.B)[row]
	if err = m.manager.Div(rhs, p, rhs); err != nil {
		return false, err
	}
	for k := range cells {
		if cells[k].Column != column {
			v := &cells[k].Value
			if err = m.manager.Div(v, p, v); err != nil {
				return false, err
			}
		}
	}
	if err = m.manager.SetInt64(p, 1); err != nil {
		return false, err
	}
	return true, nil
}

func (m *RationalCoreSolverBase) PivotToReducedCostsTableau(row, column uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if uint64(column) >= uint64(len(m.D.Data)) {
		return nil
	}
	a := &m.D.Data[column]
	zero, e := m.manager.IsZero(a)
	if e != nil {
		return e
	}
	if zero {
		return nil
	}
	if uint64(row) >= uint64(len(m.A.Rows)) {
		return ErrBounds
	}
	m.pivotWork = NewIndexedRationalVector(m.manager, 2)
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	// No child owner is called in this arithmetic segment.
	err = func() error {
		for k := range m.A.Rows[row] {
			c := &m.A.Rows[row][k]
			if c.Column == column {
				continue
			}
			if uint64(c.Column) >= uint64(len(m.D.Data)) {
				return ErrBounds
			}
			tmp, product := &m.pivotWork.Data[0], &m.pivotWork.Data[1]
			if e := etaRationalProduct(m.manager, tmp, product, a, &c.Value); e != nil {
				return e
			}
			dst := &m.D.Data[c.Column]
			if e := m.manager.Sub(dst, product, dst); e != nil {
				return e
			}
			if e := m.manager.Del(product); e != nil {
				return e
			}
			*product = numeral.NewRational()
		}
		return m.manager.SetInt64(a, 0)
	}()
	if err = m.calls.after(m.manager, before, err); err != nil {
		return err
	}
	if err = m.pivotWork.Close(); err != nil {
		return err
	}
	m.pivotWork = nil
	return nil
}

// PivotColumnTableau normalizes a row, moves its column link to the front,
// then eliminates the other links from the back. Basis membership is unchanged.
func (m *RationalCoreSolverBase) PivotColumnTableau(column, row uint32) (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	ok, err = m.DivideRowByPivot(row, column)
	if err != nil || !ok {
		return false, err
	}
	if uint64(column) >= uint64(len(m.A.Columns)) {
		return false, ErrBounds
	}
	cells := m.A.Columns[column]
	pivot := -1
	for k, c := range cells {
		if c.Row == row {
			pivot = k
			break
		}
	}
	if pivot < 0 {
		return false, nil
	}
	if pivot != 0 {
		if sourceDebugEnabled && len(cells) <= 1 {
			return false, ErrAssertion
		}
		c := cells[0]
		cells[0], cells[pivot] = cells[pivot], c
		if uint64(cells[0].Offset) >= uint64(len(m.A.Rows[row])) {
			return false, ErrBounds
		}
		m.A.Rows[row][cells[0].Offset].Offset = 0
		if uint64(c.Row) >= uint64(len(m.A.Rows)) || uint64(c.Offset) >= uint64(len(m.A.Rows[c.Row])) {
			return false, ErrBounds
		}
		m.A.Rows[c.Row][c.Offset].Offset = uint32(pivot)
	}
	for len(m.A.Columns[column]) > 1 {
		links := m.A.Columns[column]
		c := links[len(links)-1]
		if sourceDebugEnabled && c.Row == row {
			return false, ErrAssertion
		}
		// End core capture before the matrix's owning elimination call. Its
		// source bool denotes a nonempty target, not successful arithmetic.
		nonempty, e := m.A.PivotRowToRow(row, c, column)
		if e != nil {
			return false, e
		}
		if !nonempty {
			return false, nil
		}
		if m.PivotedRows != nil {
			if err = m.PivotedRows.Insert(c.Row); err != nil {
				return false, err
			}
		}
	}
	if m.Settings.SimplexStrategy == StrategyTableauCosts {
		if err = m.PivotToReducedCostsTableau(row, column); err != nil {
			return false, err
		}
	}
	return true, nil
}
