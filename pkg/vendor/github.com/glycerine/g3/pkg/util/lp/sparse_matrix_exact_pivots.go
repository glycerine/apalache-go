package lp

// Source: src/util/lp/sparse_matrix.hpp:197-226,837-905,1059-1087.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalSparseMatrix) SetMaxInRow(row uint32) (err error) {
	b, err := m.beginWork(4)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	return m.setMaxInRow(row)
}
func (m *RationalSparseMatrix) clearPivotValues() error {
	for i := 3; i >= 0; i-- {
		if err := m.manager.Del(&m.work.Data[i]); err != nil {
			return err
		}
		m.work.Data[i] = numeral.NewRational()
	}
	return nil
}
func (m *RationalSparseMatrix) setMaxInRow(row uint32) error {
	if uint64(row) >= uint64(len(m.Rows)) {
		return ErrBounds
	}
	if len(m.Rows[row]) == 0 {
		return nil
	}
	max, candidate := &m.work.Data[0], &m.work.Data[1]
	if err := m.manager.Copy(max, &m.Rows[row][0].Value); err != nil {
		return err
	}
	if err := m.manager.Abs(max); err != nil {
		return err
	}
	index := uint32(0)
	for i := 1; i < len(m.Rows[row]); i++ {
		if err := m.manager.Copy(candidate, &m.Rows[row][i].Value); err != nil {
			return err
		}
		if err := m.manager.Abs(candidate); err != nil {
			return err
		}
		cmp, err := m.manager.Compare(max, candidate)
		if err != nil {
			return err
		}
		if cmp < 0 {
			if err = m.manager.Copy(max, candidate); err != nil {
				return err
			}
			index = uint32(i)
		}
		if err = m.manager.Del(candidate); err != nil {
			return err
		}
		*candidate = numeral.NewRational()
	}
	if err := m.putMaxFirst(row, index); err != nil {
		return err
	}
	return m.clearPivotValues()
}
func (m *RationalSparseMatrix) putMaxFirst(row, index uint32) error {
	if index == 0 {
		return nil
	}
	if uint64(row) >= uint64(len(m.Rows)) || uint64(index) >= uint64(len(m.Rows[row])) {
		return ErrBounds
	}
	max, start := &m.Rows[row][index], &m.Rows[row][0]
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
	// Source indexed_value t = *max, then the two owning assignments. The
	// max's old metadata accompanies the actual lexical coefficient copy.
	oldIndex, oldOther := max.Index, max.Other
	tmp := &m.work.Data[2]
	if err = m.manager.Copy(tmp, &max.Value); err != nil {
		return err
	}
	if err = m.assignIndexed(max, start); err != nil {
		return err
	}
	if err = m.manager.Copy(&start.Value, tmp); err != nil {
		return err
	}
	start.Index, start.Other = oldIndex, oldOther
	if err = m.manager.Del(tmp); err != nil {
		return err
	}
	*tmp = numeral.NewRational()
	return nil
}
func (m *RationalSparseMatrix) PrepareForFactorization() (err error) {
	b, err := m.beginWork(4)
	if err != nil {
		return err
	}
	defer m.finishWork(b, &err)
	for j := range m.Columns {
		m.Columns[j].ShortenedMarkovitz = 0
	}
	for i := m.Dimension(); i > 0; {
		i--
		if err = m.setMaxInRow(i); err != nil {
			return err
		}
	}
	if !m.pivotQueue.IsEmpty() {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	value := &m.work.Data[3]
	for i := uint32(0); i < m.Dimension(); i++ {
		if uint64(i) >= uint64(len(m.Rows)) {
			return ErrBounds
		}
		for k := range m.Rows[i] {
			v := &m.Rows[i][k]
			// Source for(auto iv:rh) copies the owning indexed_value.
			if err = m.manager.Copy(value, &v.Value); err != nil {
				return err
			}
			if uint64(v.Index) >= uint64(len(m.Columns)) {
				return ErrBounds
			}
			score := uint32(len(m.Rows[i])) * (uint32(len(m.Columns[v.Index].Values)) - 1)
			if err = m.pivotQueue.Enqueue(i, v.Index, score); err != nil {
				return err
			}
			if err = m.manager.Del(value); err != nil {
				return err
			}
			*value = numeral.NewRational()
		}
	}
	return nil
}
func (m *RationalSparseMatrix) pivotScore(row, column uint32) (uint32, error) {
	if uint64(row) >= uint64(len(m.Rows)) || uint64(column) >= uint64(len(m.Columns)) {
		return 0, ErrBounds
	}
	// auto col_header = m_columns[j] makes actual owning coefficient copies.
	// Native storage omits vector internals; numeric copies remain observable.
	values := m.Columns[column].Values
	base := len(m.work.Data)
	for range values {
		m.work.Data = append(m.work.Data, numeral.NewRational())
	}
	for i := range values {
		if err := m.manager.Copy(&m.work.Data[base+i], &values[i].Value); err != nil {
			return 0, err
		}
	}
	score := uint32(len(m.Rows[row])) * (uint32(len(values)) - m.Columns[column].ShortenedMarkovitz - 1)
	for i := range values {
		if err := m.manager.Del(&m.work.Data[base+i]); err != nil {
			return 0, err
		}
		m.work.Data[base+i] = numeral.NewRational()
	}
	m.work.Data = m.work.Data[:base]
	return score, nil
}
func (m *RationalSparseMatrix) elementTooSmall(row, column uint32, partial int32) (int, error) {
	if uint64(row) >= uint64(len(m.Rows)) || len(m.Rows[row]) == 0 {
		return 0, ErrBounds
	}
	values := m.Rows[row]
	if column == values[0].Index {
		return 0, nil
	}
	max, candidate := &m.work.Data[0], &m.work.Data[1]
	if err := m.manager.Copy(max, &values[0].Value); err != nil {
		return 0, err
	}
	if err := m.manager.Abs(max); err != nil {
		return 0, err
	}
	for i := 1; i < len(values); i++ {
		if values[i].Index != column {
			continue
		}
		if err := m.manager.Copy(candidate, &values[i].Value); err != nil {
			return 0, err
		}
		if err := m.manager.Abs(candidate); err != nil {
			return 0, err
		}
		factor := numeral.NewRational()
		factor.Num = numeral.NewInt(partial)
		if err := etaRationalProduct(m.manager, &m.work.Data[2], &m.work.Data[3], candidate, &factor); err != nil {
			return 0, err
		}
		cmp, err := m.manager.Compare(&m.work.Data[3], max)
		if err != nil {
			return 0, err
		}
		if err = m.clearPivotValues(); err != nil {
			return 0, err
		}
		if cmp < 0 {
			return 1, nil
		}
		return 0, nil
	}
	return 2, m.clearPivotValues()
}
func (m *RationalSparseMatrix) recoverPivots(rejected []IndexPair) error {
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
func (m *RationalSparseMatrix) GetPivotForColumn(i, j *uint32, partial int32, k uint32) (ok bool, err error) {
	b, err := m.beginWork(4)
	if err != nil {
		return false, err
	}
	defer m.finishWork(b, &err)
	var rejected []IndexPair
	for !m.pivotQueue.IsEmpty() {
		p, e := m.pivotQueue.Dequeue()
		if e != nil {
			return false, e
		}
		*i, *j = p.I, p.J
		ri, e := m.AdjustRowInverse(*i)
		if e != nil {
			return false, e
		}
		if ri < k {
			continue
		}
		cj, e := m.AdjustColumnInverse(*j)
		if e != nil {
			return false, e
		}
		if cj < k {
			continue
		}
		small, e := m.elementTooSmall(*i, *j, partial)
		if e != nil {
			return false, e
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
