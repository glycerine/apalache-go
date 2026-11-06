package lp

// Source: src/util/lp/sparse_matrix.hpp:619-675.

// processYIndex marks before descending; processColumn marks after descending.
// The latter consumes the retained upper-triangular (acyclic) dependency graph.
func (m *FloatSparseMatrix) processYIndex(j uint32, out *[]uint32) error {
	if uint64(j) >= uint64(len(m.Processed)) {
		return ErrBounds
	}
	if m.Processed[j] {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	m.Processed[j] = true
	row, err := m.AdjustRow(j)
	if err != nil {
		return err
	}
	if uint64(row) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	for _, v := range m.Rows[row] {
		i, err := m.AdjustColumnInverse(v.Index)
		if err != nil {
			return err
		}
		if i == j {
			continue
		}
		if uint64(i) >= uint64(len(m.Processed)) {
			return ErrBounds
		}
		if !m.Processed[i] {
			if err = m.processYIndex(i, out); err != nil {
				return err
			}
		}
	}
	*out = append(*out, j)
	return nil
}
func (m *FloatSparseMatrix) processColumn(j uint32, out *[]uint32) error {
	if uint64(j) >= uint64(len(m.Processed)) {
		return ErrBounds
	}
	if m.Processed[j] {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
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
		if uint64(i) >= uint64(len(m.Processed)) {
			return ErrBounds
		}
		if !m.Processed[i] {
			if err = m.processColumn(i, out); err != nil {
				return err
			}
		}
	}
	m.Processed[j] = true
	*out = append(*out, j)
	return nil
}
func (m *FloatSparseMatrix) createGraph(index []uint32, out *[]uint32) error {
	for _, i := range index {
		if uint64(i) >= uint64(len(m.Processed)) {
			return ErrBounds
		}
		if m.Processed[i] {
			continue
		}
		if err := m.processColumn(i, out); err != nil {
			return err
		}
	}
	for _, i := range *out {
		if uint64(i) >= uint64(len(m.Processed)) {
			return ErrBounds
		}
		m.Processed[i] = false
	}
	return nil
}
func (m *FloatSparseMatrix) extendActiveRows(index []uint32, out *[]uint32) error {
	for _, i := range index {
		if uint64(i) >= uint64(len(m.Processed)) {
			return ErrBounds
		}
		if m.Processed[i] {
			continue
		}
		if err := m.processYIndex(i, out); err != nil {
			return err
		}
	}
	for _, i := range *out {
		if uint64(i) >= uint64(len(m.Processed)) {
			return ErrBounds
		}
		m.Processed[i] = false
	}
	return nil
}
