package lp

// Source: src/util/lp/sparse_matrix.hpp:837-905,1059-1087.

import "math"

func (m *FloatSparseMatrix) PrepareForFactorization() error {
	for j := range m.Columns {
		m.Columns[j].ShortenedMarkovitz = 0
	}
	for i := m.Dimension(); i > 0; {
		i--
		if err := m.SetMaxInRow(i); err != nil {
			return err
		}
	}
	if !m.pivotQueue.IsEmpty() {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for i := uint32(0); i < m.Dimension(); i++ {
		if uint64(i) >= uint64(len(m.Rows)) {
			return ErrBounds
		}
		for _, v := range m.Rows[i] {
			if uint64(v.Index) >= uint64(len(m.Columns)) {
				return ErrBounds
			}
			score := uint32(len(m.Rows[i])) * (uint32(len(m.Columns[v.Index].Values)) - 1)
			if err := m.pivotQueue.Enqueue(i, v.Index, score); err != nil {
				return err
			}
		}
	}
	return nil
}
func (m *FloatSparseMatrix) pivotScore(row, column uint32) (uint32, error) {
	if uint64(row) >= uint64(len(m.Rows)) || uint64(column) >= uint64(len(m.Columns)) {
		return 0, ErrBounds
	}
	return uint32(len(m.Rows[row])) * (uint32(len(m.Columns[column].Values)) - m.Columns[column].ShortenedMarkovitz - 1), nil
}
func (m *FloatSparseMatrix) elementTooSmall(row, column uint32, partial int32) (int, error) {
	if uint64(row) >= uint64(len(m.Rows)) || len(m.Rows[row]) == 0 {
		return 0, ErrBounds
	}
	values := m.Rows[row]
	if column == values[0].Index {
		return 0, nil
	}
	max := math.Abs(values[0].Value)
	for k := 1; k < len(values); k++ {
		if values[k].Index == column {
			if math.Abs(values[k].Value)*float64(partial) < max {
				return 1, nil
			}
			return 0, nil
		}
	}
	return 2, nil
}
func (m *FloatSparseMatrix) recoverPivots(rejected []IndexPair) error {
	for _, p := range rejected {
		score, err := m.pivotScore(p.I, p.J)
		if err != nil {
			return err
		}
		if err = m.pivotQueue.Enqueue(p.I, p.J, score); err != nil {
			return err
		}
	}
	return nil
}

// GetPivotForColumn publishes each dequeued physical pair immediately. Accepted
// pairs become logical indices only after rejected candidates are restored.
// Exhaustion preserves the last dequeued pair (or the input if already empty).
func (m *FloatSparseMatrix) GetPivotForColumn(i, j *uint32, partial int32, k uint32) (bool, error) {
	var rejected []IndexPair
	for !m.pivotQueue.IsEmpty() {
		p, err := m.pivotQueue.Dequeue()
		if err != nil {
			return false, err
		}
		*i, *j = p.I, p.J
		ri, err := m.AdjustRowInverse(*i)
		if err != nil {
			return false, err
		}
		if ri < k {
			continue
		}
		cj, err := m.AdjustColumnInverse(*j)
		if err != nil {
			return false, err
		}
		if cj < k {
			continue
		}
		small, err := m.elementTooSmall(*i, *j, partial)
		if err != nil {
			return false, err
		}
		if small == 0 {
			if err = m.recoverPivots(rejected); err != nil {
				return false, err
			}
			*i, *j = ri, cj
			return true, nil
		}
		if small != 2 {
			rejected = append(rejected, p)
		}
	}
	return false, m.recoverPivots(rejected)
}
