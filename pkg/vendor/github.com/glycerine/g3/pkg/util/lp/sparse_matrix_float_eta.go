package lp

// Source: src/util/lp/sparse_matrix.hpp:1119-1167.

// FillEtaMatrix constructs the eta for logical column j. A unit column returns
// nil,true; a small diagonal returns nil,false. On an index error after creation,
// eta retains the completed prefix. Entries above j do not contribute to eta.
func (m *FloatSparseMatrix) FillEtaMatrix(j uint32) (eta *FloatEtaMatrix, ok bool, err error) {
	column, err := m.AdjustColumn(j)
	if err != nil {
		return nil, false, err
	}
	if uint64(column) >= uint64(len(m.Columns)) {
		return nil, false, ErrBounds
	}
	values := m.Columns[column].Values
	unit := true
	for _, v := range values {
		i, e := m.AdjustRowInverse(v.Index)
		if e != nil {
			return nil, false, e
		}
		if i > j || (i == j && v.Value != 1) {
			unit = false
			break
		}
	}
	if unit {
		return nil, true, nil
	}
	eta = NewFloatEtaMatrix(j, m.Dimension())
	for _, v := range values {
		i, e := m.AdjustRowInverse(v.Index)
		if e != nil {
			return eta, false, e
		}
		if i < j {
			continue
		}
		if i > j {
			if err = eta.Append(i, -v.Value); err != nil {
				return eta, false, err
			}
		} else if !eta.SetDiagonal(v.Value) {
			return nil, false, nil
		}
	}
	err = eta.DivideByDiagonal()
	return eta, err == nil, err
}
