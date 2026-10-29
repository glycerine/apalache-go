package lp

// Source: src/util/lp/sparse_matrix.h:220-237,372-374,412-429;
// src/util/lp/sparse_matrix.hpp:737-743. Retained LU consumers.

// NonzeroCount counts stored row entries, including explicit zeros/duplicates.
// It is independent of the active-element counter used during factorization.
func (m *FloatSparseMatrix) NonzeroCount() (uint32, error) {
	var count uint32
	for i := m.Dimension(); i > 0; {
		i--
		if uint64(i) >= uint64(len(m.Rows)) {
			return count, ErrBounds
		}
		count += uint32(len(m.Rows[i]))
	}
	return count, nil
}

func (m *FloatSparseMatrix) MultiplyRight(p *Permutation) error {
	return m.ColumnPermutation.MultiplyRight(p)
}

func (m *FloatSparseMatrix) MultiplyReverseLeft(p *Permutation) error {
	return m.RowPermutation.MultiplyReverseLeft(p)
}

// Resize extends a factorized basis with an identity block. Existing headers,
// queue state and workspace entries are retained in source publication order.
func (m *FloatSparseMatrix) Resize(n uint32) error {
	old := m.Dimension()
	if n < old {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for j := old; j < n; j++ {
		m.Rows = append(m.Rows, nil)
		m.Columns = append(m.Columns, FloatSparseColumn{})
	}
	m.pivotQueue.Resize(n)
	m.RowPermutation.Resize(n)
	m.ColumnPermutation.Resize(n)
	if uint64(n) <= uint64(len(m.WorkPivot)) {
		m.WorkPivot = m.WorkPivot[:n]
	} else {
		m.WorkPivot = append(m.WorkPivot, make([]int32, int(n)-len(m.WorkPivot))...)
	}
	if uint64(n) <= uint64(len(m.Processed)) {
		m.Processed = m.Processed[:n]
	} else {
		m.Processed = append(m.Processed, make([]bool, int(n)-len(m.Processed))...)
	}
	for j := old; j < n; j++ {
		if err := m.AddNewElement(j, j, 1); err != nil {
			return err
		}
	}
	return nil
}
