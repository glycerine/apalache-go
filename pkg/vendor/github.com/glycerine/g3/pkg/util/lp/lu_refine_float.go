package lp

// Source: src/util/lp/lu.hpp:313-478, retained double/double refinement.

func (m *FloatLU) findRowError(yc, y []float64, basis []uint32) error {
	for i := m.Dim; i > 0; {
		i--
		if uint64(i) >= uint64(len(basis)) {
			return ErrBounds
		}
		value, err := m.A.DotColumn(y, basis[i])
		if err != nil {
			return err
		}
		if uint64(i) >= uint64(len(yc)) {
			return ErrBounds
		}
		yc[i] -= value
	}
	return nil
}

func addFloatRowCorrection(yc, y []float64) error {
	for i := len(y); i > 0; {
		i--
		if i >= len(yc) {
			return ErrBounds
		}
		y[i] += yc[i]
	}
	return nil
}

func (m *FloatLU) resetRowMarkers(size uint32) error {
	if err := m.ii.Clear(); err != nil {
		return err
	}
	return m.ii.Resize(size)
}

func (m *FloatLU) findIndexedRowError(y *IndexedFloatVector, heading []int32, s *Settings) error {
	if err := m.resetRowMarkers(y.DataSize()); err != nil {
		return err
	}
	for _, k := range y.Index {
		if uint64(k) >= uint64(len(m.A.Rows)) || uint64(k) >= uint64(len(y.Data)) {
			return ErrBounds
		}
		yk := y.Data[k]
		for _, c := range m.A.Rows[k] {
			if uint64(c.Column) >= uint64(len(heading)) {
				return ErrBounds
			}
			hj := heading[c.Column]
			if hj < 0 {
				continue
			}
			j := uint32(hj)
			if uint64(j) >= uint64(len(m.ii.Data)) {
				return ErrBounds
			}
			if m.ii.Data[j] == 0 {
				if err := m.ii.SetValue(1, j); err != nil {
					return err
				}
			}
			if uint64(j) >= uint64(len(m.yCopy.Data)) {
				return ErrBounds
			}
			m.yCopy.Data[j] -= c.Value * yk
		}
	}
	for _, i := range m.yCopy.Index {
		if uint64(i) >= uint64(len(m.ii.Data)) {
			return ErrBounds
		}
		if m.ii.Data[i] == 0 {
			if err := m.ii.SetValue(1, i); err != nil {
				return err
			}
		}
	}
	m.yCopy.Index = m.yCopy.Index[:0]
	for _, k := range m.ii.Index {
		if uint64(k) >= uint64(len(m.yCopy.Data)) {
			return ErrBounds
		}
		v := m.yCopy.Data[k]
		if s.FloatDropSmall(v) {
			m.yCopy.Data[k] = 0
		} else if err := m.yCopy.SetValue(v, k); err != nil {
			return err
		}
	}
	return nil
}

func (m *FloatLU) addIndexedRowCorrection(y *IndexedFloatVector) error {
	if err := m.resetRowMarkers(y.DataSize()); err != nil {
		return err
	}
	for _, i := range y.Index {
		if err := m.ii.SetValue(1, i); err != nil {
			return err
		}
	}
	for _, i := range m.yCopy.Index {
		if uint64(i) >= uint64(len(y.Data)) || uint64(i) >= uint64(len(m.yCopy.Data)) || uint64(i) >= uint64(len(m.ii.Data)) {
			return ErrBounds
		}
		y.Data[i] += m.yCopy.Data[i]
		if m.ii.Data[i] == 0 {
			if err := m.ii.SetValue(1, i); err != nil {
				return err
			}
		}
	}
	y.Index = y.Index[:0]
	for _, i := range m.ii.Index {
		if uint64(i) >= uint64(len(y.Data)) {
			return ErrBounds
		}
		v := y.Data[i]
		if !FloatEpsilonSmall(v, 1e-14) {
			y.Index = append(y.Index, i)
		} else if v != 0 {
			y.Data[i] = 0
		}
	}
	return nil
}

// SolveYBWithErrorCheck solves y*B=rhs and applies one residual correction.
// The residual uses the original borrowed source and caller's basis order.
func (m *FloatLU) SolveYBWithErrorCheck(y []float64, basis []uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	m.yCopy.Data = append(m.yCopy.Data[:0], y...)
	if err = m.SolveYB(y); err != nil {
		return err
	}
	if err = m.findRowError(m.yCopy.Data, y, basis); err != nil {
		return err
	}
	if err = m.SolveYB(m.yCopy.Data); err != nil {
		return err
	}
	if err = addFloatRowCorrection(m.yCopy.Data, y); err != nil {
		return err
	}
	m.yCopy.ClearAll()
	return nil
}

// SolveYBWithErrorCheckIndexed selects sparse or dense correction using the
// source's strict unsigned ratio and the full source matrix column count.
func (m *FloatLU) SolveYBWithErrorCheckIndexed(y *IndexedFloatVector, heading []int32, basis []uint32, s *Settings) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer m.finish(&err)
	if luSparseColumnRoute(y.Size(), m.A.ColumnCount(), 120) {
		if err = m.yCopy.Assign(y); err != nil {
			return err
		}
		if err = m.SolveYBIndexed(y); err != nil {
			return err
		}
		if !luSparseColumnRoute(y.Size(), m.A.ColumnCount(), 120) {
			if err = m.findRowError(m.yCopy.Data, y.Data, basis); err != nil {
				return err
			}
			if err = m.SolveYB(m.yCopy.Data); err != nil {
				return err
			}
			if err = addFloatRowCorrection(m.yCopy.Data, y.Data); err != nil {
				return err
			}
			y.RestoreIndex()
			m.yCopy.ClearAll()
		} else {
			if err = m.findIndexedRowError(y, heading, s); err != nil {
				return err
			}
			if err = m.SolveYBIndexed(m.yCopy); err != nil {
				return err
			}
			if err = m.addIndexedRowCorrection(y); err != nil {
				return err
			}
		}
		return nil
	}
	if err = m.SolveYBWithErrorCheck(y.Data, basis); err != nil {
		return err
	}
	y.RestoreIndex()
	return nil
}
