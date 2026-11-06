package lp

// Source: src/util/lp/square_dense_submatrix.h/.hpp, double/double LU instance.

import "math"

// FloatSquareDenseSubmatrix stores the combined dense elimination and tail
// block. Parent is borrowed and must remain live through tail applications.
// Permutations are independent of the parent's permutations. Do not copy.
type FloatSquareDenseSubmatrix struct {
	IndexStart, Dim                   uint32
	Data                              []float64
	Parent                            *FloatSparseMatrix
	RowPermutation, ColumnPermutation *Permutation
	WorkVector                        *IndexedFloatVector
}

func NewFloatSquareDenseSubmatrix(parent *FloatSparseMatrix, start uint32) (*FloatSquareDenseSubmatrix, error) {
	m := &FloatSquareDenseSubmatrix{IndexStart: start, Dim: parent.Dimension() - start, Parent: parent,
		RowPermutation: NewPermutation(parent.Dimension()), ColumnPermutation: NewPermutation(parent.Dimension()), WorkVector: NewIndexedFloatVector(0)}
	if start > parent.Dimension() {
		return m, ErrBounds
	}
	m.Data = make([]float64, m.Dim*m.Dim)
	offset := uint32(0) - start
	for i := start; i < parent.Dimension(); i++ {
		row, err := parent.AdjustRow(i)
		if err != nil {
			return m, err
		}
		if uint64(row) >= uint64(len(parent.Rows)) {
			return m, ErrBounds
		}
		for _, v := range parent.Rows[row] {
			j, err := parent.AdjustColumnInverse(v.Index)
			if err != nil {
				return m, err
			}
			if j < start {
				if err := sourceAssertionFailure(); err != nil {
					return m, err
				}
			}
			k := offset + j
			if uint64(k) >= uint64(len(m.Data)) {
				return m, ErrBounds
			}
			m.Data[k] = v.Value
		}
		offset += m.Dim
	}
	return m, nil
}
func (m *FloatSquareDenseSubmatrix) IsDense() bool { return true }
func (m *FloatSquareDenseSubmatrix) AdjustColumn(j uint32) (uint32, error) {
	if j >= m.ColumnPermutation.Size() {
		return j, nil
	}
	return m.ColumnPermutation.Reverse(j)
}
func (m *FloatSquareDenseSubmatrix) AdjustColumnInverse(j uint32) (uint32, error) {
	if j >= m.ColumnPermutation.Size() {
		return j, nil
	}
	return m.ColumnPermutation.Value(j)
}
func (m *FloatSquareDenseSubmatrix) AdjustRow(i uint32) (uint32, error) {
	if i >= m.RowPermutation.Size() {
		return i, nil
	}
	return m.RowPermutation.Value(i)
}
func (m *FloatSquareDenseSubmatrix) AdjustRowInverse(i uint32) (uint32, error) {
	if i >= m.RowPermutation.Size() {
		return i, nil
	}
	return m.RowPermutation.Reverse(i)
}
func (m *FloatSquareDenseSubmatrix) cell(i, j uint32) (*float64, error) {
	if i < m.IndexStart || i >= m.Parent.Dimension() || j < m.IndexStart {
		if err := sourceAssertionFailure(); err != nil {
			return nil, err
		}
	}
	j, err := m.AdjustColumn(j)
	if err != nil {
		return nil, err
	}
	k := (i-m.IndexStart)*m.Dim + j - m.IndexStart
	if uint64(k) >= uint64(len(m.Data)) {
		return nil, ErrBounds
	}
	return &m.Data[k], nil
}
func (m *FloatSquareDenseSubmatrix) SwapColumns(i, j uint32) error {
	if i == j {
		return nil
	}
	return m.ColumnPermutation.TransposeLeft(i, j)
}
func (m *FloatSquareDenseSubmatrix) FindPivotColumn(i uint32) (int32, error) {
	if i < m.IndexStart {
		if err := sourceAssertionFailure(); err != nil {
			return -1, err
		}
	}
	best, maximum := int32(-1), float64(0)
	row := (i - m.IndexStart) * m.Dim
	for k := i; k < m.Parent.Dimension(); k++ {
		col, err := m.AdjustColumn(k)
		if err != nil {
			return best, err
		}
		offset := row + col - m.IndexStart
		if uint64(offset) >= uint64(len(m.Data)) {
			return best, ErrBounds
		}
		v := math.Abs(m.Data[offset])
		if v > maximum {
			best, maximum = int32(k), v
		}
	}
	return best, nil
}
func (m *FloatSquareDenseSubmatrix) DivideRowByPivot(i uint32) error {
	pj, err := m.AdjustColumn(i)
	if err != nil {
		return err
	}
	offset := (i - m.IndexStart) * m.Dim
	k := offset + pj - m.IndexStart
	if uint64(k) >= uint64(len(m.Data)) {
		return ErrBounds
	}
	pivot := m.Data[k]
	if pivot == 0 {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	for j := m.IndexStart; j < m.Parent.Dimension(); j++ {
		if uint64(offset) >= uint64(len(m.Data)) {
			return ErrBounds
		}
		if j == pj {
			m.Data[offset] = 1 / pivot
		} else {
			m.Data[offset] /= pivot
		}
		offset++
	}
	return nil
}
func (m *FloatSquareDenseSubmatrix) PivotRowToRow(i, row uint32, s *Settings) error {
	if i >= row {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	pj, err := m.AdjustColumn(i)
	if err != nil {
		return err
	}
	po, ro := (i-m.IndexStart)*m.Dim, (row-m.IndexStart)*m.Dim
	pk, rk := po+pj-m.IndexStart, ro+pj-m.IndexStart
	if uint64(pk) >= uint64(len(m.Data)) || uint64(rk) >= uint64(len(m.Data)) {
		return ErrBounds
	}
	pivot, multiplier := m.Data[pk], m.Data[rk]
	if pivot == 0 {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	m.Data[rk] = -multiplier * pivot
	for j := m.IndexStart; j < m.Parent.Dimension(); j++ {
		if j != pj {
			if uint64(ro) >= uint64(len(m.Data)) || uint64(po) >= uint64(len(m.Data)) {
				return ErrBounds
			}
			x := m.Data[ro] - m.Data[po]*multiplier
			if s.FloatDropSmall(x) {
				m.Data[ro] = 0
			} else {
				m.Data[ro] = x
			}
		}
		ro++
		po++
	}
	return nil
}
func (m *FloatSquareDenseSubmatrix) Pivot(i uint32, s *Settings) error {
	if err := m.DivideRowByPivot(i); err != nil {
		return err
	}
	for k := i + 1; k < m.Parent.Dimension(); k++ {
		if err := m.PivotRowToRow(i, k, s); err != nil {
			return err
		}
	}
	return nil
}
func (m *FloatSquareDenseSubmatrix) UpdateExistingParentRow(i uint32, s *Settings) error {
	ai, err := m.Parent.AdjustRow(i)
	if err != nil {
		return err
	}
	if uint64(ai) >= uint64(len(m.Parent.Rows)) {
		return ErrBounds
	}
	diagonal := false
	for k := uint32(0); uint64(k) < uint64(len(m.Parent.Rows[ai])); {
		iv := &m.Parent.Rows[ai][k]
		j, err := m.Parent.AdjustColumnInverse(iv.Index)
		if err != nil {
			return err
		}
		if j < i {
			if err := m.Parent.RemoveElement(ai, k); err != nil {
				return err
			}
			continue
		}
		if j == i {
			iv.Value = 1
			other, err := m.Parent.rowOther(*iv)
			if err != nil {
				return err
			}
			other.Value = 1
			diagonal = true
		} else {
			v, err := m.cell(i, j)
			if err != nil {
				return err
			}
			if s.FloatDropSmall(*v) {
				if err := m.Parent.RemoveElement(ai, k); err != nil {
					return err
				}
				continue
			}
			iv.Value = *v
			other, err := m.Parent.rowOther(*iv)
			if err != nil {
				return err
			}
			other.Value = iv.Value
			*v = 0
		}
		k++
	}
	if !diagonal {
		aj, err := m.Parent.AdjustColumn(i)
		if err != nil {
			return err
		}
		return m.Parent.AddNewElement(ai, aj, 1)
	}
	return nil
}
func (m *FloatSquareDenseSubmatrix) PushNewParentElements(s *Settings) error {
	for i := m.IndexStart; i < m.Parent.Dimension()-1; i++ {
		ai, err := m.Parent.AdjustRow(i)
		if err != nil {
			return err
		}
		for j := i + 1; j < m.Parent.Dimension(); j++ {
			v, err := m.cell(i, j)
			if err != nil {
				return err
			}
			if !s.FloatDropSmall(*v) {
				aj, err := m.Parent.AdjustColumn(j)
				if err != nil {
					return err
				}
				if err := m.Parent.AddNewElement(ai, aj, *v); err != nil {
					return err
				}
			}
			*v = 0
		}
	}
	return nil
}
func (m *FloatSquareDenseSubmatrix) UpdateParent(s *Settings) error {
	for i := m.IndexStart; i < m.Parent.Dimension(); i++ {
		if err := m.UpdateExistingParentRow(i, s); err != nil {
			return err
		}
	}
	if err := m.PushNewParentElements(s); err != nil {
		return err
	}
	for i := m.IndexStart; i < m.Parent.Dimension(); i++ {
		row, err := m.Parent.AdjustRow(i)
		if err != nil {
			return err
		}
		if err := m.Parent.SetMaxInRow(row); err != nil {
			return err
		}
	}
	return nil
}
func (m *FloatSquareDenseSubmatrix) Conjugate(p *Permutation) error {
	if err := m.RowPermutation.MultiplyLeft(p); err != nil {
		return err
	}
	return m.ColumnPermutation.MultiplyReverseRight(p)
}
