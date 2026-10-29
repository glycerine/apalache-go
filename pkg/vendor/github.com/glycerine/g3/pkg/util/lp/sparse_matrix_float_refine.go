package lp

// Source: src/util/lp/sparse_matrix.hpp:535-584.

func (m *FloatSparseMatrix) findErrorUYIndexed(original, y *IndexedFloatVector, active []uint32) error {
	for _, i := range active {
		dot, err := m.DotProductWithRow(i, y.Data)
		if err != nil {
			return err
		}
		// Residuals must not be rounded before the correction solve.
		if err := original.Add(i, -dot); err != nil {
			return err
		}
	}
	return nil
}

func addFloatIndexedDelta(delta, y *IndexedFloatVector) error {
	for _, i := range delta.Index {
		if uint64(i) >= uint64(len(delta.Data)) {
			return ErrBounds
		}
		if err := y.Add(i, delta.Data[i]); err != nil {
			return err
		}
	}
	return nil
}

// DoubleSolveUYIndexed solves U*x=y and applies one residual correction.
// Sparse correction retains traversal order; dense correction rebuilds the
// index from all data slots. Final cleanup uses the vector's fixed tolerance.
func (m *FloatSparseMatrix) DoubleSolveUYIndexed(y *IndexedFloatVector, s *Settings) error {
	// The source copy constructor copies every dense slot, unlike assignment.
	original := &IndexedFloatVector{
		Data:  append([]float64(nil), y.Data...),
		Index: append([]uint32(nil), y.Index...),
	}
	var active []uint32
	if err := m.SolveUYIndexed(y, s, &active); err != nil {
		return err
	}
	if err := m.findErrorUYIndexed(original, y, active); err != nil {
		return err
	}
	// Both source products use unsigned arithmetic, including wraparound.
	if uint32(len(original.Index))*120 < 32*m.Dimension() {
		active = active[:0]
		if err := m.SolveUYIndexed(original, s, &active); err != nil {
			return err
		}
		if err := addFloatIndexedDelta(original, y); err != nil {
			return err
		}
		return y.CleanUp()
	}
	if err := m.SolveUY(original.Data); err != nil {
		return err
	}
	for i := m.Dimension(); i > 0; {
		i--
		if uint64(i) >= uint64(len(y.Data)) {
			return ErrBounds
		}
		y.Data[i] += original.Data[i]
	}
	y.RestoreIndex()
	return nil
}
