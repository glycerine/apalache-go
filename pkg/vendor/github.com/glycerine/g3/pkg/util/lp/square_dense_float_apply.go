package lp

// Source: src/util/lp/square_dense_submatrix.h:182-215;
// src/util/lp/square_dense_submatrix.hpp:175-197,211-299,325-351.

func (m *FloatSquareDenseSubmatrix) rowProduct(i uint32, w []float64) (float64, error) {
	if i < m.IndexStart {
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
	}
	offset, sum := (i-m.IndexStart)*m.Dim, float64(0)
	for j := uint32(0); j < m.Dim; j++ {
		col, err := m.AdjustColumnInverse(m.IndexStart + j)
		if err != nil {
			return sum, err
		}
		if uint64(offset+j) >= uint64(len(m.Data)) || uint64(col) >= uint64(len(w)) {
			return sum, ErrBounds
		}
		sum += m.Data[offset+j] * w[col]
	}
	return sum, nil
}
func (m *FloatSquareDenseSubmatrix) columnProduct(j uint32, w []float64) (float64, error) {
	if j < m.IndexStart {
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
	}
	offset, sum := j-m.IndexStart, float64(0)
	for i := uint32(0); i < m.Dim; i++ {
		row, err := m.AdjustRowInverse(m.IndexStart + i)
		if err != nil {
			return sum, err
		}
		if uint64(offset) >= uint64(len(m.Data)) || uint64(row) >= uint64(len(w)) {
			return sum, ErrBounds
		}
		sum += m.Data[offset] * w[row]
		offset += m.Dim
	}
	return sum, nil
}
func (m *FloatSquareDenseSubmatrix) ApplyLeft(w []float64) error {
	t := make([]float64, m.Parent.Dimension())
	for i := uint32(0); i < m.IndexStart; i++ {
		row, err := m.AdjustRowInverse(i)
		if err != nil {
			return err
		}
		col, err := m.AdjustColumnInverse(i)
		if err != nil {
			return err
		}
		if uint64(row) >= uint64(len(t)) || uint64(col) >= uint64(len(w)) {
			return ErrBounds
		}
		t[row] = w[col]
	}
	for i := m.IndexStart; i < m.Parent.Dimension(); i++ {
		row, err := m.AdjustRowInverse(i)
		if err != nil {
			return err
		}
		v, err := m.rowProduct(i, w)
		if err != nil {
			return err
		}
		if uint64(row) >= uint64(len(t)) {
			return ErrBounds
		}
		t[row] = v
	}
	for i := uint32(0); i < m.Parent.Dimension(); i++ {
		if uint64(i) >= uint64(len(w)) {
			return ErrBounds
		}
		w[i] = t[i]
	}
	return nil
}
func (m *FloatSquareDenseSubmatrix) ApplyRight(w []float64) error {
	t := make([]float64, len(w))
	end := m.IndexStart + m.Dim
	for j := uint32(0); j < m.IndexStart; j++ {
		col, err := m.AdjustColumnInverse(j)
		if err != nil {
			return err
		}
		row, err := m.AdjustRowInverse(j)
		if err != nil {
			return err
		}
		if uint64(col) >= uint64(len(t)) || uint64(row) >= uint64(len(w)) {
			return ErrBounds
		}
		t[col] = w[row]
	}
	for j := end; j < m.Parent.Dimension(); j++ {
		col, err := m.AdjustColumnInverse(j)
		if err != nil {
			return err
		}
		row, err := m.AdjustRowInverse(j)
		if err != nil {
			return err
		}
		if uint64(col) >= uint64(len(t)) || uint64(row) >= uint64(len(w)) {
			return ErrBounds
		}
		t[col] = w[row]
	}
	for j := m.IndexStart; j < end; j++ {
		col, err := m.AdjustColumnInverse(j)
		if err != nil {
			return err
		}
		v, err := m.columnProduct(j, w)
		if err != nil {
			return err
		}
		if uint64(col) >= uint64(len(t)) {
			return ErrBounds
		}
		t[col] = v
	}
	copy(w, t)
	return nil
}
func (m *FloatSquareDenseSubmatrix) ApplyLeftToT(w *IndexedFloatVector, s *Settings) error {
	t := make([]float64, m.Parent.Dimension())
	for _, k := range w.Index {
		j, err := m.AdjustColumn(k)
		if err != nil {
			return err
		}
		if uint64(k) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		if j < m.IndexStart || j >= m.IndexStart+m.Dim {
			row, err := m.AdjustRowInverse(j)
			if err != nil {
				return err
			}
			if uint64(row) >= uint64(len(t)) {
				return ErrBounds
			}
			t[row] = w.Data[k]
		} else {
			for i := uint32(0); i < m.Dim; i++ {
				row, err := m.AdjustRowInverse(m.IndexStart + i)
				if err != nil {
					return err
				}
				offset := i*m.Dim + j - m.IndexStart
				if uint64(row) >= uint64(len(t)) || uint64(offset) >= uint64(len(m.Data)) {
					return ErrBounds
				}
				t[row] += m.Data[offset] * w.Data[k]
			}
		}
	}
	w.Index = w.Index[:0]
	for i := uint32(0); i < m.Parent.Dimension(); i++ {
		v := t[i]
		if !s.FloatDropSmall(v) {
			w.Index = append(w.Index, i)
			if uint64(i) >= uint64(len(w.Data)) {
				return ErrBounds
			}
			w.Data[i] = v
		} else {
			if uint64(i) >= uint64(len(w.Data)) {
				return ErrBounds
			}
			w.Data[i] = 0
		}
	}
	return nil
}
func (m *FloatSquareDenseSubmatrix) ApplyRightIndexed(w *IndexedFloatVector) error {
	if err := m.WorkVector.Resize(w.DataSize()); err != nil {
		return err
	}
	if err := m.WorkVector.Clear(); err != nil {
		return err
	}
	end := m.IndexStart + m.Dim
	for _, k := range w.Index {
		j, err := m.AdjustRow(k)
		if err != nil {
			return err
		}
		if uint64(k) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		if j < m.IndexStart || j >= end {
			col, err := m.AdjustColumnInverse(j)
			if err != nil {
				return err
			}
			if err := m.WorkVector.SetValue(w.Data[k], col); err != nil {
				return err
			}
		} else {
			offset := (j - m.IndexStart) * m.Dim
			for col := m.IndexStart; col < end; col++ {
				adj, err := m.AdjustColumnInverse(col)
				if err != nil {
					return err
				}
				if uint64(offset) >= uint64(len(m.Data)) {
					return ErrBounds
				}
				if err := m.WorkVector.Add(adj, m.Data[offset]*w.Data[k]); err != nil {
					return err
				}
				offset++
			}
		}
	}
	if err := m.WorkVector.CleanUp(); err != nil {
		return err
	}
	return w.Assign(m.WorkVector)
}

var _ FloatTailMatrix = (*FloatSquareDenseSubmatrix)(nil)
