package lp

// Source: src/util/lp/sparse_matrix.hpp:756-835, retained mpq coefficients.

import "github.com/glycerine/g3/pkg/util/numeral"

// replacementAbsCompare owns both actual abs temporaries until comparison ends.
// Slots0..3 are reserved for existing maximum/append helpers; slot4 holds w_at_i.
func (m *RationalSparseMatrix) replacementAbsCompare(a, b *numeral.Rational) (int, error) {
	left, right := &m.work.Data[5], &m.work.Data[6]
	if err := m.manager.Copy(left, a); err != nil {
		return 0, err
	}
	if err := m.manager.Abs(left); err != nil {
		return 0, err
	}
	if err := m.manager.Copy(right, b); err != nil {
		return 0, err
	}
	if err := m.manager.Abs(right); err != nil {
		return 0, err
	}
	cmp, err := m.manager.Compare(left, right)
	if err != nil {
		return 0, err
	}
	if err = m.manager.Del(right); err != nil {
		return 0, err
	}
	*right = numeral.NewRational()
	if err = m.manager.Del(left); err != nil {
		return 0, err
	}
	*left = numeral.NewRational()
	return cmp, nil
}
func (m *RationalSparseMatrix) removeElementsNotInWAndUpdateCommon(column uint32, w *IndexedRationalVector) error {
	if uint64(column) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	for k := len(m.Columns[column].Values); k > 0; {
		k--
		iv := &m.Columns[column].Values[k]
		physical, offset := iv.Index, iv.Other
		logical, err := m.AdjustRowInverse(physical)
		if err != nil {
			return err
		}
		if uint64(logical) >= uint64(len(w.Data)) || uint64(physical) >= uint64(len(m.Rows)) {
			return ErrBounds
		}
		row := m.Rows[physical]
		if uint64(offset) >= uint64(len(row)) {
			return ErrBounds
		}
		value := &w.Data[logical]
		zero, err := m.manager.IsZero(value)
		if err != nil {
			return err
		}
		if zero {
			if err = m.removeAt(physical, offset, column, row[offset].Other); err != nil {
				return err
			}
			if offset == 0 {
				if err = m.setMaxInRow(physical); err != nil {
					return err
				}
			}
		} else {
			if offset == 0 {
				cmp, err := m.replacementAbsCompare(value, &row[0].Value)
				if err != nil {
					return err
				}
				if err = m.manager.Copy(&iv.Value, value); err != nil {
					return err
				}
				if err = m.setIndexedValue(&row[0].Value, &iv.Value, 7); err != nil {
					return err
				}
				if cmp < 0 {
					if err = m.setMaxInRow(physical); err != nil {
						return err
					}
				}
			} else {
				if err = m.manager.Copy(&iv.Value, value); err != nil {
					return err
				}
				if err = m.setIndexedValue(&row[offset].Value, &iv.Value, 7); err != nil {
					return err
				}
				cmp, err := m.replacementAbsCompare(value, &row[0].Value)
				if err != nil {
					return err
				}
				if cmp > 0 {
					if err = m.putMaxFirst(physical, offset); err != nil {
						return err
					}
				}
			}
			if err = m.manager.Copy(value, &m.zero); err != nil {
				return err
			}
		}
	}
	return nil
}
func (m *RationalSparseMatrix) addNewElementsOfWAndClear(column uint32, w *IndexedRationalVector) error {
	for _, i := range w.Index {
		if uint64(i) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		value := &m.work.Data[4]
		if err := m.manager.Copy(value, &w.Data[i]); err != nil {
			return err
		}
		zero, err := m.manager.IsZero(value)
		if err != nil {
			return err
		}
		if !zero {
			// Exact drop tolerance is another zero predicate, independent of settings.
			drop, err := m.manager.IsZero(value)
			if err != nil {
				return err
			}
			if !drop {
				row, err := m.AdjustRow(i)
				if err != nil {
					return err
				}
				if err = m.addNewElement(row, column, value); err != nil {
					return err
				}
				values := m.Rows[row]
				if len(values) == 0 {
					if err = sourceAssertionFailure(); err != nil {
						return err
					}
					return ErrBounds
				}
				cmp, err := m.replacementAbsCompare(value, &values[0].Value)
				if err != nil {
					return err
				}
				if cmp > 0 {
					if err = m.putMaxFirst(row, uint32(len(values)-1)); err != nil {
						return err
					}
				}
			}
			if err = m.manager.Copy(&w.Data[i], &m.zero); err != nil {
				return err
			}
		}
		if err = m.manager.Del(value); err != nil {
			return err
		}
		*value = numeral.NewRational()
	}
	w.Index = w.Index[:0]
	return nil
}

// ReplaceColumn consumes a borrowed logical column with the same manager. The
// matrix owns lexical copies, comparisons and every new numerical call; w owns
// its completed/partial slot values. On failure close the matrix before w.
func (m *RationalSparseMatrix) ReplaceColumn(column uint32, w *IndexedRationalVector, _ *Settings) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if w.manager != m.manager {
		return ErrExactVectorOwner
	}
	if w.calls.pending() {
		return ErrExactVectorPending
	}
	b, err := m.beginWork(8)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	physical, err := m.AdjustColumn(column)
	if err != nil {
		return err
	}
	if err = m.removeElementsNotInWAndUpdateCommon(physical, w); err != nil {
		return err
	}
	return m.addNewElementsOfWAndClear(physical, w)
}
