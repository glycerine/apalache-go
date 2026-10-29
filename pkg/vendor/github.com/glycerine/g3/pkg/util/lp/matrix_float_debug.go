//go:build g3debug

package lp

// Source: src/util/lp/matrix.hpp:27-72; dense_matrix.hpp:183-197.

import "math"

func MultiplyFloatDebugMatrices(a, b FloatDebugMatrix) (*FloatDenseMatrix, error) {
	if a == nil || b == nil {
		return nil, ErrBounds
	}
	if a.ColumnCount() != b.RowCount() {
		return nil, ErrAssertion
	}
	result, err := NewFloatDenseMatrix(a.RowCount(), b.ColumnCount())
	if err != nil {
		return result, err
	}
	for i := uint32(0); i < result.rows; i++ {
		for j := uint32(0); j < result.columns; j++ {
			value := 0.0
			for k := uint32(0); k < a.ColumnCount(); k++ {
				x, err := a.Element(i, k)
				if err != nil {
					return result, err
				}
				y, err := b.Element(k, j)
				if err != nil {
					return result, err
				}
				value += x * y
			}
			if err = result.SetElement(i, j, value); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

// FloatDebugMatricesEqual preserves the literal >1e-6 test: equality at the
// tolerance and unordered NaN differences both compare equal in source.
func FloatDebugMatricesEqual(a, b FloatDebugMatrix) (bool, error) {
	if a == nil || b == nil {
		return false, ErrBounds
	}
	if b.RowCount() != a.RowCount() || b.ColumnCount() != a.ColumnCount() {
		return false, nil
	}
	for i := uint32(0); i < a.RowCount(); i++ {
		for j := uint32(0); j < a.ColumnCount(); j++ {
			x, err := a.Element(i, j)
			if err != nil {
				return false, err
			}
			y, err := b.Element(i, j)
			if err != nil {
				return false, err
			}
			if math.Abs(x-y) > 0.000001 {
				return false, nil
			}
		}
	}
	return true, nil
}
