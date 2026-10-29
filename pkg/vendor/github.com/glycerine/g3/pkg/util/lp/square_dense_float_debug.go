//go:build g3debug

package lp

// Source: src/util/lp/square_dense_submatrix.h:222-227;
// src/util/lp/square_dense_submatrix.hpp:354-363.

func (m *FloatSquareDenseSubmatrix) RowCount() uint32    { return m.Parent.Dimension() }
func (m *FloatSquareDenseSubmatrix) ColumnCount() uint32 { return m.RowCount() }
func (m *FloatSquareDenseSubmatrix) Element(i, j uint32) (float64, error) {
	i, err := m.AdjustRow(i)
	if err != nil {
		return 0, err
	}
	j, err = m.AdjustColumn(j)
	if err != nil {
		return 0, err
	}
	if i < m.IndexStart || j < m.IndexStart {
		if i == j {
			return 1, nil
		}
		return 0, nil
	}
	offset := (i-m.IndexStart)*m.Dim + j - m.IndexStart
	if uint64(offset) >= uint64(len(m.Data)) {
		return 0, ErrBounds
	}
	return m.Data[offset], nil
}
