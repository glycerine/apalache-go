package lp

// Source: src/util/lp/eta_matrix.h/.hpp, concrete double,double instance.

// FloatEtaMatrix is identity with one replaced column. SetDiagonal must precede
// numerical use; the C++ double constructor does not initialize that member.
// Column retains literal sparse terms and their order. Do not shallow-copy.
type FloatEtaMatrix struct {
	length, column uint32
	Column         SparseFloatVector
	Diagonal       float64
}

func NewFloatEtaMatrix(column, length uint32) *FloatEtaMatrix {
	return &FloatEtaMatrix{column: column, length: length}
}
func (m *FloatEtaMatrix) IsDense() bool       { return false }
func (m *FloatEtaMatrix) ColumnIndex() uint32 { return m.column }
func (m *FloatEtaMatrix) SetDiagonal(value float64) bool {
	m.Diagonal = value
	return !FloatEpsilonSmall(value, 1e-12)
}
func (m *FloatEtaMatrix) Append(row uint32, value float64) error {
	if row == m.column {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	m.Column.Append(row, value)
	return nil
}
func (m *FloatEtaMatrix) DivideByDiagonal() error { return m.Column.Divide(m.Diagonal) }
func (m *FloatEtaMatrix) RowCount() uint32        { return m.length }
func (m *FloatEtaMatrix) ColumnCount() uint32     { return m.length }
func (m *FloatEtaMatrix) Element(i, j uint32) (float64, error) {
	if j == m.column {
		if i == j {
			return 1 / m.Diagonal, nil
		}
		return m.Column.At(i), nil
	}
	if i == j {
		return 1, nil
	}
	return 0, nil
}
func (m *FloatEtaMatrix) ApplyLeft(w []float64) error {
	if uint64(m.column) >= uint64(len(w)) {
		return ErrBounds
	}
	for _, entry := range m.Column.Data {
		if uint64(entry.Index) >= uint64(len(w)) {
			return ErrBounds
		}
		w[entry.Index] += w[m.column] * entry.Value
	}
	w[m.column] /= m.Diagonal
	return nil
}
func (m *FloatEtaMatrix) ApplyLeftToT(w *IndexedFloatVector, s *Settings) error {
	if uint64(m.column) >= uint64(len(w.Data)) {
		return ErrBounds
	}
	anchor := w.Data[m.column]
	if anchor == 0 {
		return nil
	}
	w.Data[m.column] /= m.Diagonal
	if s.FloatDropSmall(w.Data[m.column]) {
		w.Data[m.column] = 0
		w.EraseFromIndex(m.column)
	}
	for _, entry := range m.Column.Data {
		i := entry.Index
		if uint64(i) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		if w.Data[i] == 0 {
			w.Data[i] = anchor * entry.Value
			if s.FloatDropSmall(w.Data[i]) {
				w.Data[i] = 0
				continue
			}
			w.Index = append(w.Index, i)
		} else {
			w.Data[i] += anchor * entry.Value
			if s.FloatDropSmall(w.Data[i]) {
				w.Data[i] = 0
				w.EraseFromIndex(i)
			}
		}
	}
	return nil
}
func (m *FloatEtaMatrix) ApplyRight(w []float64) error {
	if uint64(m.column) >= uint64(len(w)) {
		return ErrBounds
	}
	value := w[m.column] / m.Diagonal
	for _, entry := range m.Column.Data {
		if uint64(entry.Index) >= uint64(len(w)) {
			return ErrBounds
		}
		value += w[entry.Index] * entry.Value
	}
	w[m.column] = value
	return nil
}
func (m *FloatEtaMatrix) ApplyRightIndexed(w *IndexedFloatVector) error {
	if len(w.Index) == 0 {
		return nil
	}
	if uint64(m.column) >= uint64(len(w.Data)) {
		return ErrBounds
	}
	w.Data[m.column] /= m.Diagonal
	wasInIndex := w.Data[m.column] != 0
	for _, entry := range m.Column.Data {
		if uint64(entry.Index) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		w.Data[m.column] += w.Data[entry.Index] * entry.Value
	}
	if !FloatEpsilonSmall(w.Data[m.column], 1e-14) {
		if !wasInIndex {
			w.Index = append(w.Index, m.column)
		}
	} else {
		if wasInIndex {
			w.EraseFromIndex(m.column)
		}
		w.Data[m.column] = 0
	}
	return nil
}
func (m *FloatEtaMatrix) Conjugate(p *Permutation) error {
	column, err := p.Reverse(m.column)
	if err != nil {
		return err
	}
	m.column = column
	for i := range m.Column.Data {
		index, err := p.Reverse(m.Column.Data[i].Index)
		if err != nil {
			return err
		}
		m.Column.Data[i].Index = index
	}
	return nil
}

var _ FloatTailMatrix = (*FloatEtaMatrix)(nil)
