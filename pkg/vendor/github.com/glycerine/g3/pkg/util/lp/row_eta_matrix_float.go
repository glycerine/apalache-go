package lp

// Source: src/util/lp/row_eta_matrix.h/.hpp, concrete double,double instance.

// FloatRowEtaMatrix is identity plus the literal sparse terms of one row.
// The source row_start constructor argument is never read by retained bodies.
// Sparse left applications accumulate locally before publishing that row.
type FloatRowEtaMatrix struct {
	row, length uint32
	Row         SparseFloatVector
}

func NewFloatRowEtaMatrix(_ uint32, row, length uint32) *FloatRowEtaMatrix {
	return &FloatRowEtaMatrix{row: row, length: length}
}
func (m *FloatRowEtaMatrix) IsDense() bool       { return false }
func (m *FloatRowEtaMatrix) RowIndex() uint32    { return m.row }
func (m *FloatRowEtaMatrix) RowCount() uint32    { return m.length }
func (m *FloatRowEtaMatrix) ColumnCount() uint32 { return m.length }
func (m *FloatRowEtaMatrix) Append(index uint32, value float64) error {
	if index == m.row {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	m.Row.Append(index, value)
	return nil
}
func (m *FloatRowEtaMatrix) Element(i, j uint32) (float64, error) {
	if i == m.row {
		if i == j {
			return 1, nil
		}
		return m.Row.At(j), nil
	}
	if i == j {
		return 1, nil
	}
	return 0, nil
}
func (m *FloatRowEtaMatrix) ApplyLeft(w []float64) error {
	if uint64(m.row) >= uint64(len(w)) {
		return ErrBounds
	}
	for _, entry := range m.Row.Data {
		if uint64(entry.Index) >= uint64(len(w)) {
			return ErrBounds
		}
		w[m.row] += w[entry.Index] * entry.Value
	}
	return nil
}
func (m *FloatRowEtaMatrix) ApplyLeftToT(w *IndexedFloatVector, s *Settings) error {
	if uint64(m.row) >= uint64(len(w.Data)) {
		return ErrBounds
	}
	value := w.Data[m.row]
	wasZero := value == 0
	for _, entry := range m.Row.Data {
		if uint64(entry.Index) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		value += w.Data[entry.Index] * entry.Value
	}
	if !s.FloatDropSmall(value) {
		if wasZero {
			w.Index = append(w.Index, m.row)
		}
		w.Data[m.row] = value
	} else if !wasZero {
		w.Data[m.row] = 0
		// Source erase(find(...)) has no end guard. Preserve the preceding zero
		// write and return a typed error at that unsupported erase position.
		found := false
		for _, index := range w.Index {
			if index == m.row {
				found = true
				break
			}
		}
		if !found {
			return ErrBounds
		}
		w.EraseFromIndex(m.row)
	}
	return nil
}
func (m *FloatRowEtaMatrix) ApplyRight(w []float64) error {
	if uint64(m.row) >= uint64(len(w)) {
		return ErrBounds
	}
	if w[m.row] == 0 {
		return nil
	}
	for _, entry := range m.Row.Data {
		if uint64(entry.Index) >= uint64(len(w)) {
			return ErrBounds
		}
		w[entry.Index] += w[m.row] * entry.Value
	}
	return nil
}
func (m *FloatRowEtaMatrix) ApplyRightIndexed(w *IndexedFloatVector) error {
	if uint64(m.row) >= uint64(len(w.Data)) {
		return ErrBounds
	}
	if w.Data[m.row] == 0 {
		return nil
	}
	for _, entry := range m.Row.Data {
		j := entry.Index
		if uint64(j) >= uint64(len(w.Data)) {
			return ErrBounds
		}
		wasZero := w.Data[j] == 0
		w.Data[j] += w.Data[m.row] * entry.Value
		if wasZero {
			if !FloatEpsilonSmall(w.Data[j], 1e-14) {
				w.Index = append(w.Index, j)
			} else {
				w.Data[j] = 0
			}
		} else if FloatEpsilonSmall(w.Data[j], 1e-14) {
			w.EraseFromIndex(j)
			w.Data[j] = 0
		}
	}
	return nil
}
func (m *FloatRowEtaMatrix) Conjugate(p *Permutation) error {
	row, err := p.Reverse(m.row)
	if err != nil {
		return err
	}
	m.row = row
	columns := make([]uint32, len(m.Row.Data))
	for i, entry := range m.Row.Data {
		columns[i] = entry.Index
	}
	for i := len(columns); i > 0; {
		i--
		index, err := p.Reverse(columns[i])
		if err != nil {
			return err
		}
		m.Row.Data[i].Index = index
	}
	return nil
}

var _ FloatTailMatrix = (*FloatRowEtaMatrix)(nil)
