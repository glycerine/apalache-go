package lp

// Source: src/util/lp/sparse_matrix.hpp:171-257,278-330,420-432,799-809.

import "math"

// AddNewElement uses physical indices and publishes row before column, unlike
// the basis constructor. It preserves zeros and duplicate coordinates.
func (m *FloatSparseMatrix) AddNewElement(row, column uint32, value float64) error {
	if uint64(row) >= uint64(len(m.Rows)) || uint64(column) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	ro, co := uint32(len(m.Rows[row])), uint32(len(m.Columns[column].Values))
	m.Rows[row] = append(m.Rows[row], FloatIndexedValue{value, column, co})
	m.Columns[column].Values = append(m.Columns[column].Values, FloatIndexedValue{value, row, ro})
	m.active++
	return nil
}

func (m *FloatSparseMatrix) rowOther(v FloatIndexedValue) (*FloatIndexedValue, error) {
	if uint64(v.Index) >= uint64(len(m.Columns)) || uint64(v.Other) >= uint64(len(m.Columns[v.Index].Values)) {
		return nil, ErrBounds
	}
	return &m.Columns[v.Index].Values[v.Other], nil
}
func (m *FloatSparseMatrix) columnOther(v FloatIndexedValue) (*FloatIndexedValue, error) {
	if uint64(v.Index) >= uint64(len(m.Rows)) || uint64(v.Other) >= uint64(len(m.Rows[v.Index])) {
		return nil, ErrBounds
	}
	return &m.Rows[v.Index][v.Other], nil
}

// RemoveElement uses the reciprocal column cell's row offset, literally as the
// source wrapper does. Tail assignments precede backlink repair and tail pops.
func (m *FloatSparseMatrix) RemoveElement(row, offset uint32) error {
	if uint64(row) >= uint64(len(m.Rows)) || uint64(offset) >= uint64(len(m.Rows[row])) {
		return ErrBounds
	}
	v := m.Rows[row][offset]
	c, err := m.rowOther(v)
	if err != nil {
		return err
	}
	return m.removeAt(row, c.Other, v.Index, v.Other)
}
func (m *FloatSparseMatrix) removeAt(row, ro, column, co uint32) error {
	if uint64(row) >= uint64(len(m.Rows)) || uint64(ro) >= uint64(len(m.Rows[row])) || uint64(column) >= uint64(len(m.Columns)) || uint64(co) >= uint64(len(m.Columns[column].Values)) {
		return ErrBounds
	}
	rLast, cLast := len(m.Rows[row])-1, len(m.Columns[column].Values)-1
	if int(co) != cLast {
		m.Columns[column].Values[co] = m.Columns[column].Values[cLast]
		other, err := m.columnOther(m.Columns[column].Values[co])
		if err != nil {
			return err
		}
		other.Other = co
		if int(ro) != rLast {
			m.Rows[row][ro] = m.Rows[row][rLast]
			other, err = m.rowOther(m.Rows[row][ro])
			if err != nil {
				return err
			}
			other.Other = ro
		}
	} else if int(ro) != rLast {
		m.Rows[row][ro] = m.Rows[row][rLast]
		other, err := m.rowOther(m.Rows[row][ro])
		if err != nil {
			return err
		}
		other.Other = ro
	}
	m.Columns[column].Values = m.Columns[column].Values[:cLast]
	m.Rows[row] = m.Rows[row][:rLast]
	m.active-- // Source only promises this count during refactorization.
	return nil
}

func (m *FloatSparseMatrix) SetMaxInRow(row uint32) error {
	if uint64(row) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	values := m.Rows[row]
	if len(values) == 0 {
		return nil
	}
	max, index := math.Abs(values[0].Value), uint32(0)
	for i := 1; i < len(values); i++ {
		a := math.Abs(values[i].Value)
		if a > max {
			max, index = a, uint32(i)
		}
	}
	return m.putMaxFirst(row, index)
}
func (m *FloatSparseMatrix) putMaxFirst(row, index uint32) error {
	if index == 0 {
		return nil
	}
	if uint64(row) >= uint64(len(m.Rows)) || uint64(index) >= uint64(len(m.Rows[row])) {
		return ErrBounds
	}
	max, start := m.Rows[row][index], m.Rows[row][0]
	c, err := m.rowOther(max)
	if err != nil {
		return err
	}
	c.Other = 0
	c, err = m.rowOther(start)
	if err != nil {
		return err
	}
	c.Other = index
	m.Rows[row][index], m.Rows[row][0] = start, max
	return nil
}

func (m *FloatSparseMatrix) scanRowAndRemovePivot(row, pivot uint32) error {
	if uint64(row) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	for j := 0; j < len(m.Rows[row]); {
		v := m.Rows[row][j]
		if v.Index == pivot {
			if err := m.RemoveElement(row, uint32(j)); err != nil {
				return err
			}
		} else {
			if uint64(v.Index) >= uint64(len(m.WorkPivot)) {
				return ErrBounds
			}
			m.WorkPivot[v.Index] = int32(j)
			j++
		}
	}
	return nil
}

// PivotRowToRow removes the target's pivot column, then adds alpha times all
// remaining pivot-row terms. Alpha is borrowed, as in the retained eta caller.
// Cancellation offsets are insertion-sorted and removed in descending order.
func (m *FloatSparseMatrix) PivotRowToRow(pivot uint32, alpha *float64, target uint32, s *Settings) (bool, error) {
	if pivot >= m.Dimension() || target >= m.Dimension() {
		if err := sourceAssertionFailure(); err != nil {
			return false, err
		}
		return false, ErrBounds
	}
	if pivot == target {
		if err := sourceAssertionFailure(); err != nil {
			return false, err
		}
	}
	column, err := m.AdjustColumn(pivot)
	if err != nil {
		return false, err
	}
	row, err := m.AdjustRow(pivot)
	if err != nil {
		return false, err
	}
	dest, err := m.AdjustRow(target)
	if err != nil {
		return false, err
	}
	if err = m.scanRowAndRemovePivot(dest, column); err != nil {
		return false, err
	}
	if uint64(row) >= uint64(len(m.Rows)) {
		return false, ErrBounds
	}
	previous := len(m.Rows[dest])
	var zeros []uint32
	for k := 0; k < len(m.Rows[row]); k++ {
		v := m.Rows[row][k]
		if v.Index == column {
			continue
		}
		product := *alpha * v.Value
		if uint64(v.Index) >= uint64(len(m.WorkPivot)) {
			return false, ErrBounds
		}
		offset := m.WorkPivot[v.Index]
		if offset == -1 {
			if !s.FloatDropSmall(product) {
				if err = m.AddNewElement(dest, v.Index, product); err != nil {
					return false, err
				}
			}
		} else {
			if offset < 0 || int64(offset) >= int64(len(m.Rows[dest])) {
				return false, ErrBounds
			}
			value := &m.Rows[dest][offset]
			value.Value += product
			if s.FloatDropSmall(value.Value) {
				zeros = append(zeros, uint32(offset))
				for j := len(zeros) - 1; j > 0 && zeros[j] <= zeros[j-1]; j-- {
					zeros[j], zeros[j-1] = zeros[j-1], zeros[j]
				}
			} else {
				other, e := m.rowOther(*value)
				if e != nil {
					return false, e
				}
				other.Value = value.Value
			}
		}
	}
	for k := 0; k < previous; k++ {
		j := m.Rows[dest][k].Index
		if uint64(j) >= uint64(len(m.WorkPivot)) {
			return false, ErrBounds
		}
		m.WorkPivot[j] = -1
	}
	for k := len(zeros) - 1; k >= 0; k-- {
		if err = m.RemoveElement(dest, zeros[k]); err != nil {
			return false, err
		}
		if len(m.Rows[dest]) == 0 {
			return false, nil
		}
	}
	if err = m.SetMaxInRow(dest); err != nil {
		return false, err
	}
	return len(m.Rows[dest]) != 0, nil
}

// DivideRow uses logical row indices and a live divisor reference. Small results
// become stored positive zeros; entries and reciprocal positions are retained.
func (m *FloatSparseMatrix) DivideRow(row uint32, divisor *float64, s *Settings) error {
	if s.FloatZeroSmall(*divisor) {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	r, err := m.AdjustRow(row)
	if err != nil {
		return err
	}
	if uint64(r) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	for k := range m.Rows[r] {
		v := &m.Rows[r][k]
		v.Value /= *divisor
		if s.FloatDropSmall(v.Value) {
			v.Value = 0
		}
		other, err := m.rowOther(*v)
		if err != nil {
			return err
		}
		other.Value = v.Value
	}
	return nil
}
