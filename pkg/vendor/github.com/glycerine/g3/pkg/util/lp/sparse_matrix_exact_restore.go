package lp

// Source: src/util/lp/sparse_matrix.hpp:337-400, retained mpq coefficients.

import "github.com/glycerine/g3/pkg/util/numeral"

// SetRowFromWorkVectorAndCleanNotAdjusted consumes logical columns into a
// physical row. Exact drop tolerance means zero. The matrix owns lexical
// copies and new numerical call receipts; close it before w after failure.
func (m *RationalSparseMatrix) SetRowFromWorkVectorAndCleanNotAdjusted(row uint32, w *IndexedRationalVector, _ *Settings) (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	if w.manager != m.manager {
		return false, ErrExactVectorOwner
	}
	if w.calls.pending() {
		return false, ErrExactVectorPending
	}
	b, err := m.beginWork(5)
	if err != nil {
		return false, err
	}
	defer m.finishWork(b, &err)
	if uint64(row) >= uint64(len(m.Rows)) {
		return false, ErrBounds
	}
	value := &m.work.Data[4]
	for k := len(m.Rows[row]); k > 0; {
		k--
		iv := &m.Rows[row][k]
		column, other := iv.Index, iv.Other
		j, err := m.AdjustColumnInverse(column)
		if err != nil {
			return false, err
		}
		if uint64(j) >= uint64(len(w.Data)) {
			return false, ErrBounds
		}
		if err = m.manager.Copy(value, &w.Data[j]); err != nil {
			return false, err
		}
		zero, err := m.manager.IsZero(value)
		if err != nil {
			return false, err
		}
		if zero {
			if err = m.removeAt(row, uint32(k), column, other); err != nil {
				return false, err
			}
		} else {
			// The nested source assignment publishes the row before the column
			// setter's by-value argument and destination copies.
			if err = m.manager.Copy(&iv.Value, value); err != nil {
				return false, err
			}
			if uint64(column) >= uint64(len(m.Columns)) || uint64(other) >= uint64(len(m.Columns[column].Values)) {
				return false, ErrBounds
			}
			if err = m.setIndexedValue(&m.Columns[column].Values[other].Value, &iv.Value, 3); err != nil {
				return false, err
			}
			if err = m.manager.Copy(&w.Data[j], &m.zero); err != nil {
				return false, err
			}
		}
		if err = m.manager.Del(value); err != nil {
			return false, err
		}
		*value = numeral.NewRational()
	}
	for _, j := range w.Index {
		if uint64(j) >= uint64(len(w.Data)) {
			return false, ErrBounds
		}
		zero, err := m.manager.IsZero(&w.Data[j])
		if err != nil {
			return false, err
		}
		if zero {
			continue
		}
		column, err := m.AdjustColumn(j)
		if err != nil {
			return false, err
		}
		if err = m.addNewElement(row, column, &w.Data[j]); err != nil {
			return false, err
		}
		if err = m.manager.Copy(&w.Data[j], &m.zero); err != nil {
			return false, err
		}
	}
	w.Index = w.Index[:0]
	if len(m.Rows[row]) == 0 {
		return false, nil
	}
	if err = m.setMaxInRow(row); err != nil {
		return false, err
	}
	return true, nil
}
