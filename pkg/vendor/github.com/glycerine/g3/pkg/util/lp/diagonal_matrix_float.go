package lp

// Source: src/util/lp/lu.h:54-120; src/util/lp/lu.hpp:110-125,926-934.

// FloatDiagonalMatrix divides one coordinate by a stored diagonal coefficient.
// It is the retained one_elem_on_diag tail used by LU column updates.
type FloatDiagonalMatrix struct {
	index, length  uint32
	value, inverse float64
}

func NewFloatDiagonalMatrix(index uint32, value float64) *FloatDiagonalMatrix {
	m := &FloatDiagonalMatrix{index: index, value: value}
	if sourceDebugEnabled {
		m.inverse = 1 / value
	}
	return m
}

func (m *FloatDiagonalMatrix) IsDense() bool { return false }
func (m *FloatDiagonalMatrix) Index() uint32 { return m.index }
func (m *FloatDiagonalMatrix) ApplyLeft(w []float64) error {
	if uint64(m.index) >= uint64(len(w)) {
		return ErrBounds
	}
	w[m.index] /= m.value
	return nil
}
func (m *FloatDiagonalMatrix) ApplyRight(w []float64) error {
	if uint64(m.index) >= uint64(len(w)) {
		return ErrBounds
	}
	w[m.index] /= m.value
	return nil
}
func (m *FloatDiagonalMatrix) ApplyRightIndexed(w *IndexedFloatVector) error {
	if uint64(m.index) >= uint64(len(w.Data)) {
		return ErrBounds
	}
	if w.Data[m.index] == 0 {
		return nil
	}
	w.Data[m.index] /= m.value
	if FloatEpsilonSmall(w.Data[m.index], 1e-14) {
		w.EraseFromIndex(m.index)
		w.Data[m.index] = 0
	}
	return nil
}
func (m *FloatDiagonalMatrix) ApplyLeftToT(w *IndexedFloatVector, s *Settings) error {
	if uint64(m.index) >= uint64(len(w.Data)) {
		return ErrBounds
	}
	if w.Data[m.index] == 0 {
		return nil
	}
	w.Data[m.index] /= m.value
	if s.FloatDropSmall(w.Data[m.index]) {
		w.EraseFromIndex(m.index)
		w.Data[m.index] = 0
	}
	return nil
}
func (m *FloatDiagonalMatrix) Conjugate(p *Permutation) error {
	i, err := p.Reverse(m.index)
	if err != nil {
		return err
	}
	m.index = i
	return nil
}

var _ FloatTailMatrix = (*FloatDiagonalMatrix)(nil)
