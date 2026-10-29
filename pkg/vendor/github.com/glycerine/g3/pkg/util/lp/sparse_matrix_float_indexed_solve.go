package lp

// Source: src/util/lp/sparse_matrix.hpp:467-493,679-705.

// SolveYUIndexed solves x*U=y over the row-reachable sparse domain. Visitation
// postorder is retained in the rebuilt index, including zero/tolerance policy.
func (m *FloatSparseMatrix) SolveYUIndexed(y *IndexedFloatVector, s *Settings) error {
	var active []uint32
	if err := m.extendActiveRows(y.Index, &active); err != nil {
		return err
	}
	for k := len(active); k > 0; {
		k--
		j := active[k]
		if uint64(j) >= uint64(len(y.Data)) {
			return ErrBounds
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
			if uint64(i) >= uint64(len(y.Data)) {
				return ErrBounds
			}
			y.Data[j] -= y.Data[i] * v.Value
		}
	}
	y.Index = y.Index[:0]
	for _, j := range active {
		if uint64(j) >= uint64(len(y.Data)) {
			return ErrBounds
		}
		if !s.FloatDropSmall(y.Data[j]) {
			y.Index = append(y.Index, j)
		} else {
			y.Data[j] = 0
		}
	}
	return nil
}

// SolveUYIndexed appends graph postorder to active without clearing its caller's
// prefix, then solves U*x=y backwards through that order. On error both active
// and y retain their completed prefixes. Graph errors retain visitation flags.
func (m *FloatSparseMatrix) SolveUYIndexed(y *IndexedFloatVector, s *Settings, active *[]uint32) error {
	if err := m.createGraph(y.Index, active); err != nil {
		return err
	}
	for k := len(*active); k > 0; {
		k--
		j := (*active)[k]
		if uint64(j) >= uint64(len(y.Data)) {
			return ErrBounds
		}
		if y.Data[j] == 0 {
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
			if uint64(i) >= uint64(len(y.Data)) {
				return ErrBounds
			}
			y.Data[i] -= v.Value * y.Data[j]
		}
	}
	y.Index = y.Index[:0]
	for _, j := range *active {
		if uint64(j) >= uint64(len(y.Data)) {
			return ErrBounds
		}
		if !s.FloatDropSmall(y.Data[j]) {
			y.Index = append(y.Index, j)
		} else {
			y.Data[j] = 0
		}
	}
	return nil
}
