package lp

// Source: src/util/lp/sparse_matrix.hpp:171-196,241-257,278-330,420-432,799-809.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalSparseMatrix) beginWork(size uint32) (exactEtaBefore, error) {
	if err := m.ready(); err != nil {
		return exactEtaBefore{}, err
	}
	b, err := m.calls.begin(m.manager)
	if err != nil {
		return b, err
	}
	m.work = NewIndexedRationalVector(m.manager, size)
	return b, nil
}
func (m *RationalSparseMatrix) finishWork(b exactEtaBefore, err *error) {
	*err = m.calls.after(m.manager, b, *err)
	if *err == nil {
		*err = m.work.Close()
		if *err == nil {
			m.work = nil
		}
	}
}
func (m *RationalSparseMatrix) AddNewElement(row, column uint32, value *numeral.Rational) (err error) {
	b, err := m.beginWork(3)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	return m.addNewElement(row, column, value)
}
func (m *RationalSparseMatrix) addNewElement(row, column uint32, value *numeral.Rational) error {
	if uint64(row) >= uint64(len(m.Rows)) || uint64(column) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	ro, co := uint32(len(m.Rows[row])), uint32(len(m.Columns[column].Values))
	if err := m.appendIndexed(&m.Rows[row], value, column, co); err != nil {
		return err
	}
	if err := m.appendIndexed(&m.Columns[column].Values, value, row, ro); err != nil {
		return err
	}
	m.active++
	return nil
}
func (m *RationalSparseMatrix) rowOther(v *RationalIndexedValue) (*RationalIndexedValue, error) {
	if uint64(v.Index) >= uint64(len(m.Columns)) || uint64(v.Other) >= uint64(len(m.Columns[v.Index].Values)) {
		return nil, ErrBounds
	}
	return &m.Columns[v.Index].Values[v.Other], nil
}
func (m *RationalSparseMatrix) columnOther(v *RationalIndexedValue) (*RationalIndexedValue, error) {
	if uint64(v.Index) >= uint64(len(m.Rows)) || uint64(v.Other) >= uint64(len(m.Rows[v.Index])) {
		return nil, ErrBounds
	}
	return &m.Rows[v.Index][v.Other], nil
}

// indexed_value assignment publishes value before its index/other metadata.
func (m *RationalSparseMatrix) assignIndexed(dst, src *RationalIndexedValue) error {
	if err := m.manager.Copy(&dst.Value, &src.Value); err != nil {
		return err
	}
	dst.Index, dst.Other = src.Index, src.Other
	return nil
}
func (m *RationalSparseMatrix) RemoveElement(row, offset uint32) (err error) {
	b, err := m.beginWork(0)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	return m.removeElement(row, offset)
}
func (m *RationalSparseMatrix) removeElement(row, offset uint32) error {
	if uint64(row) >= uint64(len(m.Rows)) || uint64(offset) >= uint64(len(m.Rows[row])) {
		return ErrBounds
	}
	v := &m.Rows[row][offset]
	c, err := m.rowOther(v)
	if err != nil {
		return err
	}
	return m.removeAt(row, c.Other, v.Index, v.Other)
}
func (m *RationalSparseMatrix) removeAt(row, ro, column, co uint32) error {
	if uint64(row) >= uint64(len(m.Rows)) || uint64(ro) >= uint64(len(m.Rows[row])) || uint64(column) >= uint64(len(m.Columns)) || uint64(co) >= uint64(len(m.Columns[column].Values)) {
		return ErrBounds
	}
	rLast, cLast := len(m.Rows[row])-1, len(m.Columns[column].Values)-1
	if int(co) != cLast {
		if err := m.assignIndexed(&m.Columns[column].Values[co], &m.Columns[column].Values[cLast]); err != nil {
			return err
		}
		other, err := m.columnOther(&m.Columns[column].Values[co])
		if err != nil {
			return err
		}
		other.Other = co
		if int(ro) != rLast {
			if err = m.assignIndexed(&m.Rows[row][ro], &m.Rows[row][rLast]); err != nil {
				return err
			}
			other, err = m.rowOther(&m.Rows[row][ro])
			if err != nil {
				return err
			}
			other.Other = ro
		}
	} else if int(ro) != rLast {
		if err := m.assignIndexed(&m.Rows[row][ro], &m.Rows[row][rLast]); err != nil {
			return err
		}
		other, err := m.rowOther(&m.Rows[row][ro])
		if err != nil {
			return err
		}
		other.Other = ro
	}
	if err := m.manager.Del(&m.Columns[column].Values[cLast].Value); err != nil {
		return err
	}
	m.Columns[column].Values[cLast].Value = numeral.NewRational()
	m.Columns[column].Values = m.Columns[column].Values[:cLast]
	if err := m.manager.Del(&m.Rows[row][rLast].Value); err != nil {
		return err
	}
	m.Rows[row][rLast].Value = numeral.NewRational()
	m.Rows[row] = m.Rows[row][:rLast]
	m.active--
	return nil
}
func (m *RationalSparseMatrix) scanRowAndRemovePivot(row, pivot uint32) error {
	if uint64(row) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	for j := 0; j < len(m.Rows[row]); {
		v := &m.Rows[row][j]
		if v.Index == pivot {
			if err := m.removeElement(row, uint32(j)); err != nil {
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

// set_value(T val) copies a by-value argument before assigning the column.
func (m *RationalSparseMatrix) setIndexedValue(dst, value *numeral.Rational, slot int) error {
	arg := &m.work.Data[slot]
	if err := m.manager.Copy(arg, value); err != nil {
		return err
	}
	if err := m.manager.Copy(dst, arg); err != nil {
		return err
	}
	if err := m.manager.Del(arg); err != nil {
		return err
	}
	*arg = numeral.NewRational()
	return nil
}

// Precise drop/zero tolerance means exact zero regardless of float settings.
// Precise pivoting does not move the maximum coefficient to the row head.
func (m *RationalSparseMatrix) PivotRowToRow(pivot uint32, alpha *numeral.Rational, target uint32, _ *Settings) (ok bool, err error) {
	b, err := m.beginWork(6)
	if err != nil {
		return false, err
	}
	defer m.finishWork(b, &err)
	if pivot >= m.Dimension() || target >= m.Dimension() {
		if err = sourceAssertionFailure(); err != nil {
			return false, err
		}
		return false, ErrBounds
	}
	if pivot == target {
		if err = sourceAssertionFailure(); err != nil {
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
	product := &m.work.Data[4]
	for k := 0; k < len(m.Rows[row]); k++ {
		v := &m.Rows[row][k]
		j := v.Index
		if j == column {
			continue
		}
		if err = etaRationalProduct(m.manager, &m.work.Data[3], product, alpha, &v.Value); err != nil {
			return false, err
		}
		if uint64(j) >= uint64(len(m.WorkPivot)) {
			return false, ErrBounds
		}
		offset := m.WorkPivot[j]
		if offset == -1 {
			zero, e := m.manager.IsZero(product)
			if e != nil {
				return false, e
			}
			if !zero {
				if err = m.addNewElement(dest, j, product); err != nil {
					return false, err
				}
			}
		} else {
			if offset < 0 || int64(offset) >= int64(len(m.Rows[dest])) {
				return false, ErrBounds
			}
			value := &m.Rows[dest][offset]
			if err = m.manager.Add(&value.Value, product, &value.Value); err != nil {
				return false, err
			}
			zero, e := m.manager.IsZero(&value.Value)
			if e != nil {
				return false, e
			}
			if zero {
				zeros = append(zeros, uint32(offset))
				for j := len(zeros) - 1; j > 0 && zeros[j] <= zeros[j-1]; j-- {
					zeros[j], zeros[j-1] = zeros[j-1], zeros[j]
				}
			} else {
				other, e := m.rowOther(value)
				if e != nil {
					return false, e
				}
				if err = m.setIndexedValue(&other.Value, &value.Value, 5); err != nil {
					return false, err
				}
			}
		}
		if err = m.manager.Del(product); err != nil {
			return false, err
		}
		*product = numeral.NewRational()
	}
	for k := 0; k < previous; k++ {
		j := m.Rows[dest][k].Index
		if uint64(j) >= uint64(len(m.WorkPivot)) {
			return false, ErrBounds
		}
		m.WorkPivot[j] = -1
	}
	for k := len(zeros) - 1; k >= 0; k-- {
		if err = m.removeElement(dest, zeros[k]); err != nil {
			return false, err
		}
		if len(m.Rows[dest]) == 0 {
			return false, nil
		}
	}
	return len(m.Rows[dest]) != 0, nil
}
func (m *RationalSparseMatrix) DivideRow(row uint32, divisor *numeral.Rational, _ *Settings) (err error) {
	b, err := m.beginWork(1)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	zero, err := m.manager.IsZero(divisor)
	if err != nil {
		return err
	}
	if zero {
		if err = sourceAssertionFailure(); err != nil {
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
		if err = m.manager.Div(&v.Value, divisor, &v.Value); err != nil {
			return err
		}
		zero, err = m.manager.IsZero(&v.Value)
		if err != nil {
			return err
		}
		if zero {
			if err = m.manager.Copy(&v.Value, &m.zero); err != nil {
				return err
			}
		}
		other, e := m.rowOther(v)
		if e != nil {
			return e
		}
		if err = m.setIndexedValue(&other.Value, &v.Value, 0); err != nil {
			return err
		}
	}
	return nil
}
