package lp

// Source: src/util/lp/sparse_matrix.hpp:437-462,528-534,545-552,585-617,713-722.

// SolveYU solves x*U=y in place for unit upper triangular U. The source omits
// division by the diagonal and never reads the final anchor coordinate.
func (m *FloatSparseMatrix) SolveYU(y []float64) error {
	for i := uint32(0); i+1 < m.Dimension(); i++ {
		if uint64(i) >= uint64(len(y)) {
			return ErrBounds
		}
		if y[i] == 0 {
			continue
		}
		row, err := m.AdjustRow(i)
		if err != nil {
			return err
		}
		if uint64(row) >= uint64(len(m.Rows)) {
			return ErrBounds
		}
		for _, v := range m.Rows[row] {
			col, err := m.AdjustColumnInverse(v.Index)
			if err != nil {
				return err
			}
			if col == i {
				continue
			}
			if uint64(col) >= uint64(len(y)) {
				return ErrBounds
			}
			y[col] -= v.Value * y[i]
		}
	}
	return nil
}

// SolveUY solves U*x=y in place for unit upper triangular U, visiting columns
// backwards. Each nonzero anchor is borrowed live through its column traversal.
func (m *FloatSparseMatrix) SolveUY(y []float64) error {
	for j := m.Dimension(); j > 0; {
		j--
		if uint64(j) >= uint64(len(y)) {
			return ErrBounds
		}
		if y[j] == 0 {
			continue
		}
		col, err := m.AdjustColumn(j)
		if err != nil {
			return err
		}
		if uint64(col) >= uint64(len(m.Columns)) {
			return ErrBounds
		}
		for _, v := range m.Columns[col].Values {
			i, err := m.AdjustRowInverse(v.Index)
			if err != nil {
				return err
			}
			if i == j {
				continue
			}
			if uint64(i) >= uint64(len(y)) {
				return ErrBounds
			}
			y[i] -= v.Value * y[j]
		}
	}
	return nil
}
func (m *FloatSparseMatrix) DotProductWithRow(row uint32, y []float64) (float64, error) {
	physical, err := m.AdjustRow(row)
	if err != nil {
		return 0, err
	}
	if uint64(physical) >= uint64(len(m.Rows)) {
		return 0, ErrBounds
	}
	sum := float64(0)
	for _, v := range m.Rows[physical] {
		col, err := m.AdjustColumnInverse(v.Index)
		if err != nil {
			return sum, err
		}
		if uint64(col) >= uint64(len(y)) {
			return sum, ErrBounds
		}
		sum += v.Value * y[col]
	}
	return sum, nil
}

// DoubleSolveUY performs the retained floating solve/residual/correction pass.
// The source copies all dense slots, including trailing coordinates.
func (m *FloatSparseMatrix) DoubleSolveUY(y []float64) error {
	original := append([]float64(nil), y...)
	if err := m.SolveUY(y); err != nil {
		return err
	}
	for i := m.Dimension(); i > 0; {
		i--
		if uint64(i) >= uint64(len(original)) {
			return ErrBounds
		}
		dot, err := m.DotProductWithRow(i, y)
		if err != nil {
			return err
		}
		original[i] -= dot
	}
	if err := m.SolveUY(original); err != nil {
		return err
	}
	for i := m.Dimension(); i > 0; {
		i--
		y[i] += original[i]
	}
	return nil
}
