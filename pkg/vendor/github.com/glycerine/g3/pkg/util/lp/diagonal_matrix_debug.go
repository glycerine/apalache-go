//go:build g3debug

package lp

// Source: src/util/lp/lu.h:66-82; src/util/lp/lu.hpp:96-108.

func (m *FloatDiagonalMatrix) SetDimension(n uint32) { m.length = n }
func (m *FloatDiagonalMatrix) RowCount() uint32      { return m.length }
func (m *FloatDiagonalMatrix) ColumnCount() uint32   { return m.length }
func (m *FloatDiagonalMatrix) Element(i, j uint32) float64 {
	if i == j {
		if j == m.index {
			return m.inverse
		}
		return 1
	}
	return 0
}
