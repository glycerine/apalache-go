package lp

// Source: src/util/lp/sparse_matrix.hpp:337-400.

// SetRowFromWorkVectorAndCleanNotAdjusted replaces a physical row from logical
// column coordinates. Completed mutations remain published on error.
func (m *FloatSparseMatrix) SetRowFromWorkVectorAndCleanNotAdjusted(row uint32, w *IndexedFloatVector, s *Settings) (bool, error) {
	if uint64(row) >= uint64(len(m.Rows)) {
		return false, ErrBounds
	}
	for k := len(m.Rows[row]); k > 0; {
		k--
		iv := m.Rows[row][k]
		j, err := m.AdjustColumnInverse(iv.Index)
		if err != nil {
			return false, err
		}
		if uint64(j) >= uint64(len(w.Data)) {
			return false, ErrBounds
		}
		value := w.Data[j]
		if s.FloatDropSmall(value) {
			if err = m.removeAt(row, uint32(k), iv.Index, iv.Other); err != nil {
				return false, err
			}
			if value != 0 {
				if err = sourceAssertionFailure(); err != nil {
					return false, err
				}
			}
		} else {
			m.Rows[row][k].Value = value
			if uint64(iv.Index) >= uint64(len(m.Columns)) || uint64(iv.Other) >= uint64(len(m.Columns[iv.Index].Values)) {
				return false, ErrBounds
			}
			m.Columns[iv.Index].Values[iv.Other].Value = value
			w.Data[j] = 0
		}
	}
	for _, j := range w.Index {
		if uint64(j) >= uint64(len(w.Data)) {
			return false, ErrBounds
		}
		value := w.Data[j]
		if value == 0 {
			continue
		}
		if s.FloatDropSmall(value) {
			if err := sourceAssertionFailure(); err != nil {
				return false, err
			}
		}
		column, err := m.AdjustColumn(j)
		if err != nil {
			return false, err
		}
		if err = m.AddNewElement(row, column, value); err != nil {
			return false, err
		}
		w.Data[j] = 0
	}
	w.Index = w.Index[:0]
	if len(m.Rows[row]) == 0 {
		return false, nil
	}
	if err := m.SetMaxInRow(row); err != nil {
		return false, err
	}
	return true, nil
}
