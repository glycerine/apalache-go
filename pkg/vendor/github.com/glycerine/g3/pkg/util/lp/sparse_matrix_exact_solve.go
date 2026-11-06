package lp

// Source: src/util/lp/sparse_matrix.hpp:437-462,598-617; retained mpq/mpq and
// mpq/numeric_pair dense LU consumers in lu.hpp:175-196.

import "github.com/glycerine/g3/pkg/util/numeral"

// SolveYU solves x*U=y in place for unit upper triangular U. Matrix owns actual
// product temporaries and new numerical calls; caller owns completed/partial y.
// Close a stopped matrix before the caller's destination values.
func (m *RationalSparseMatrix) SolveYU(y []numeral.Rational) (err error) {
	b, err := m.beginWork(2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	for i := uint32(0); i+1 < m.Dimension(); i++ {
		if uint64(i) >= uint64(len(y)) {
			return ErrBounds
		}
		zero, e := m.manager.IsZero(&y[i])
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		row, e := m.AdjustRow(i)
		if e != nil {
			return e
		}
		if uint64(row) >= uint64(len(m.Rows)) {
			return ErrBounds
		}
		for k := range m.Rows[row] {
			v := &m.Rows[row][k]
			col, e := m.AdjustColumnInverse(v.Index)
			if e != nil {
				return e
			}
			if col == i {
				continue
			}
			if uint64(col) >= uint64(len(y)) {
				return ErrBounds
			}
			product := &m.work.Data[1]
			if err = etaRationalProduct(m.manager, &m.work.Data[0], product, &v.Value, &y[i]); err != nil {
				return err
			}
			if err = m.manager.Sub(&y[col], product, &y[col]); err != nil {
				return err
			}
			if err = m.manager.Del(product); err != nil {
				return err
			}
			*product = numeral.NewRational()
		}
	}
	return nil
}

// SolveUY solves U*x=y in place for unit upper triangular U. No diagonal
// division is performed; source callers provide the unit-diagonal LU workspace.
func (m *RationalSparseMatrix) SolveUY(y []numeral.Rational) (err error) {
	b, err := m.beginWork(2)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	for j := m.Dimension(); j > 0; {
		j--
		if uint64(j) >= uint64(len(y)) {
			return ErrBounds
		}
		zero, e := m.manager.IsZero(&y[j])
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		col, e := m.AdjustColumn(j)
		if e != nil {
			return e
		}
		if uint64(col) >= uint64(len(m.Columns)) {
			return ErrBounds
		}
		for k := range m.Columns[col].Values {
			v := &m.Columns[col].Values[k]
			i, e := m.AdjustRowInverse(v.Index)
			if e != nil {
				return e
			}
			if i == j {
				continue
			}
			if uint64(i) >= uint64(len(y)) {
				return ErrBounds
			}
			product := &m.work.Data[1]
			if err = etaRationalProduct(m.manager, &m.work.Data[0], product, &v.Value, &y[j]); err != nil {
				return err
			}
			if err = m.manager.Sub(&y[i], product, &y[i]); err != nil {
				return err
			}
			if err = m.manager.Del(product); err != nil {
				return err
			}
			*product = numeral.NewRational()
		}
	}
	return nil
}

// SolveUYPair is the retained mpq/numeric_pair dense right-side solve. The scalar
// left product constructs both expressions, then copies X/Y into its pair value;
// subtraction publishes X before Y. Its owning slots remain with matrix on error.
func (m *RationalSparseMatrix) SolveUYPair(y []RationalPair) (err error) {
	b, err := m.beginWork(5)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	for j := m.Dimension(); j > 0; {
		j--
		if uint64(j) >= uint64(len(y)) {
			return ErrBounds
		}
		zero, e := y[j].IsZero(m.manager)
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		col, e := m.AdjustColumn(j)
		if e != nil {
			return e
		}
		if uint64(col) >= uint64(len(m.Columns)) {
			return ErrBounds
		}
		for k := range m.Columns[col].Values {
			v := &m.Columns[col].Values[k]
			i, e := m.AdjustRowInverse(v.Index)
			if e != nil {
				return e
			}
			if i == j {
				continue
			}
			if uint64(i) >= uint64(len(y)) {
				return ErrBounds
			}
			x, yp, tmp := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2]
			px, py := &m.work.Data[3], &m.work.Data[4]
			if err = etaRationalProduct(m.manager, tmp, x, &v.Value, &y[j].X); err != nil {
				return err
			}
			if err = etaRationalProduct(m.manager, tmp, yp, &v.Value, &y[j].Y); err != nil {
				return err
			}
			if err = m.manager.Copy(px, x); err != nil {
				return err
			}
			if err = m.manager.Copy(py, yp); err != nil {
				return err
			}
			if err = m.manager.Del(yp); err != nil {
				return err
			}
			*yp = numeral.NewRational()
			if err = m.manager.Del(x); err != nil {
				return err
			}
			*x = numeral.NewRational()
			if err = m.manager.Sub(&y[i].X, px, &y[i].X); err != nil {
				return err
			}
			if err = m.manager.Sub(&y[i].Y, py, &y[i].Y); err != nil {
				return err
			}
			if err = m.manager.Del(py); err != nil {
				return err
			}
			*py = numeral.NewRational()
			if err = m.manager.Del(px); err != nil {
				return err
			}
			*px = numeral.NewRational()
		}
	}
	return nil
}
