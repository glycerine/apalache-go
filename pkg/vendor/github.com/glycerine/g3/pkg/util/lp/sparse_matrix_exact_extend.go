package lp

// Source: src/util/lp/sparse_matrix.h:220-237,372-374,412-429;
// src/util/lp/sparse_matrix.hpp:737-743. Retained mpq LU consumers.

import "github.com/glycerine/g3/pkg/util/numeral"

// NonzeroCount counts stored entries, not their numerical zero predicates.
func (m *RationalSparseMatrix) NonzeroCount() (uint32, error) {
	if err := m.ready(); err != nil {
		return 0, err
	}
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

func (m *RationalSparseMatrix) MultiplyRight(p *Permutation) error {
	if err := m.ready(); err != nil {
		return err
	}
	return m.ColumnPermutation.MultiplyRight(p)
}

func (m *RationalSparseMatrix) MultiplyReverseLeft(p *Permutation) error {
	if err := m.ready(); err != nil {
		return err
	}
	return m.RowPermutation.MultiplyReverseLeft(p)
}

// Resize adds identity entries after publishing all new headers and extents.
// Numerical failure retains completed entries and the new dimension for Close.
func (m *RationalSparseMatrix) Resize(n uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	old := m.Dimension()
	if n < old {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	b, err := m.beginWork(4)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	for j := old; j < n; j++ {
		m.Rows = append(m.Rows, nil)
		m.Columns = append(m.Columns, RationalSparseColumn{})
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
		if err = m.manager.SetInt64(&m.work.Data[3], 1); err != nil {
			return err
		}
		if err = m.addNewElement(j, j, &m.work.Data[3]); err != nil {
			return err
		}
		if err = m.manager.Del(&m.work.Data[3]); err != nil {
			return err
		}
		m.work.Data[3] = numeral.NewRational()
	}
	return nil
}
