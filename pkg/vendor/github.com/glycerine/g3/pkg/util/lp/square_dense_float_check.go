package lp

// Source: src/util/lp/square_dense_submatrix.hpp:302-323.

func (m *FloatSquareDenseSubmatrix) IsLMatrix() (bool, error) {
	if !sourceDebugEnabled {
		return true, nil
	}
	if !m.RowPermutation.IsIdentity() {
		return false, ErrAssertion
	}
	for i := uint32(0); i < m.Parent.Dimension(); i++ {
		if i < m.IndexStart {
			j, err := m.ColumnPermutation.Value(i)
			if err != nil {
				return false, err
			}
			if j != i {
				return false, ErrAssertion
			}
			continue
		}
		offset := (i - m.IndexStart) * m.Dim
		for k := uint32(0); k < m.Dim; k++ {
			j, err := m.AdjustColumnInverse(m.IndexStart + k)
			if err != nil {
				return false, err
			}
			if j >= i {
				if uint64(offset+k) >= uint64(len(m.Data)) {
					return false, ErrBounds
				}
				v := m.Data[offset+k]
				if (j > i && v != 0) || (j == i && v == 0) {
					return false, ErrAssertion
				}
			}
		}
	}
	return true, nil
}
