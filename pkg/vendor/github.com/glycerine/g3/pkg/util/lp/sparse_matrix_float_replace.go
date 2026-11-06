package lp

// Source: src/util/lp/sparse_matrix.hpp:756-835.

import "math"

func (m *FloatSparseMatrix) removeElementsNotInWAndUpdateCommon(column uint32, w *IndexedFloatVector) error {
	if uint64(column) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	for k := len(m.Columns[column].Values); k > 0; {
		k--
		iv := m.Columns[column].Values[k]
		logical, err := m.AdjustRowInverse(iv.Index)
		if err != nil {
			return err
		}
		if uint64(logical) >= uint64(len(w.Data)) || uint64(iv.Index) >= uint64(len(m.Rows)) {
			return ErrBounds
		}
		row := m.Rows[iv.Index]
		if uint64(iv.Other) >= uint64(len(row)) {
			return ErrBounds
		}
		if w.Data[logical] == 0 {
			if err = m.removeAt(iv.Index, iv.Other, column, row[iv.Other].Other); err != nil {
				return err
			}
			if iv.Other == 0 {
				if err = m.SetMaxInRow(iv.Index); err != nil {
					return err
				}
			}
		} else {
			offset := iv.Other
			if offset == 0 {
				look := math.Abs(w.Data[logical]) < math.Abs(row[0].Value)
				m.Columns[column].Values[k].Value = w.Data[logical]
				row[0].Value = m.Columns[column].Values[k].Value
				if look {
					if err = m.SetMaxInRow(iv.Index); err != nil {
						return err
					}
				}
			} else {
				m.Columns[column].Values[k].Value = w.Data[logical]
				row[offset].Value = m.Columns[column].Values[k].Value
				if math.Abs(w.Data[logical]) > math.Abs(row[0].Value) {
					if err = m.putMaxFirst(iv.Index, offset); err != nil {
						return err
					}
				}
			}
			w.Data[logical] = 0
		}
	}
	return nil
}
func (m *FloatSparseMatrix) addNewElementsOfWAndClear(column uint32, w *IndexedFloatVector, s *Settings) error {
	for _, i := range w.Index {
		if uint64(i) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		value := w.Data[i]
		if value == 0 {
			continue
		}
		if !s.FloatDropSmall(value) {
			row, err := m.AdjustRow(i)
			if err != nil {
				return err
			}
			if err = m.AddNewElement(row, column, value); err != nil {
				return err
			}
			values := m.Rows[row]
			if len(values) == 0 {
				if err = sourceAssertionFailure(); err != nil {
					return err
				}
				return ErrBounds
			}
			if math.Abs(value) > math.Abs(values[0].Value) {
				if err = m.putMaxFirst(row, uint32(len(values)-1)); err != nil {
					return err
				}
			}
		}
		w.Data[i] = 0
	}
	w.Index = w.Index[:0]
	return nil
}

// ReplaceColumn consumes the logical indexed column w. Existing nonzero entries
// are kept regardless of drop tolerance; tolerance applies only to new entries.
// Work proceeds backwards through the old column, then forwards through w.Index.
// On error, completed matrix and input-vector changes remain published.
func (m *FloatSparseMatrix) ReplaceColumn(column uint32, w *IndexedFloatVector, s *Settings) error {
	physical, err := m.AdjustColumn(column)
	if err != nil {
		return err
	}
	if err = m.removeElementsNotInWAndUpdateCommon(physical, w); err != nil {
		return err
	}
	return m.addNewElementsOfWAndClear(physical, w, s)
}
