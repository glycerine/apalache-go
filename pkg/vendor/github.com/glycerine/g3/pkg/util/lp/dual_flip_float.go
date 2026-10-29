package lp

// Source: src/util/lp/lp_dual_core_solver.h:172-180;
// lp_dual_core_solver.hpp:375-386,711-730,742-750.

func (m *FloatDualCoreSolver) SignedSpanOfBoxed(j uint32) (float64, error) {
	low, err := m.dualAtBound(j, false)
	if err != nil {
		return 0, err
	}
	span, err := m.BoundSpan(j)
	if err != nil {
		return 0, err
	}
	if low {
		return span, nil
	}
	return -span, nil
}
func (m *FloatDualCoreSolver) UpdateAWave(delta float64, j uint32) error {
	return m.A.AddColumnToVector(delta, j, m.AWave)
}
func (m *FloatDualCoreSolver) ProcessFlipped() error {
	if err := m.InitAWaveByZeros(); err != nil {
		return err
	}
	for _, j := range dualOrderedKeys(m.FlippedBoxed) {
		span, err := m.SignedSpanOfBoxed(j)
		if err != nil {
			return err
		}
		if err = m.UpdateAWave(span, j); err != nil {
			return err
		}
	}
	return nil
}
func (m *FloatDualCoreSolver) UpdateXBAfterBoundFlips() error {
	if m.Factorization == nil {
		return ErrBounds
	}
	if err := m.Factorization.SolveBy(m.AWave); err != nil {
		return err
	}
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(*m.Basis)) {
			return ErrBounds
		}
		j := (*m.Basis)[i]
		if uint64(j) >= uint64(len(*m.X)) || uint64(i) >= uint64(len(m.AWave)) {
			return ErrBounds
		}
		(*m.X)[j] -= m.AWave[i]
	}
	return nil
}
func (m *FloatDualCoreSolver) UpdateDAndXB() error {
	for _, j := range *m.NBasis {
		if uint64(j) >= uint64(len(m.D)) || uint64(j) >= uint64(len(m.PivotRow.Data)) {
			return ErrBounds
		}
		m.D[j] -= m.ThetaD * m.PivotRow.Data[j]
	}
	p := uint32(m.P)
	if uint64(p) >= uint64(len(m.D)) {
		return ErrBounds
	}
	m.D[p] = -m.ThetaD
	if len(m.FlippedBoxed) > 0 {
		if err := m.ProcessFlipped(); err != nil {
			return err
		}
		return m.UpdateXBAfterBoundFlips()
	}
	return nil
}
func (m *FloatDualCoreSolver) ApplyFlips() error {
	for _, j := range dualOrderedKeys(m.FlippedBoxed) {
		if sourceDebugEnabled {
			low, err := m.dualAtBound(j, false)
			if err != nil {
				return err
			}
			if !low {
				upper, err := m.dualAtBound(j, true)
				if err != nil {
					return err
				}
				if !upper {
					return ErrAssertion
				}
			}
		}
		low, err := m.dualAtBound(j, false)
		if err != nil {
			return err
		}
		_, bound, err := m.dualBound(j, low)
		if err != nil {
			return err
		}
		(*m.X)[j] = bound
	}
	return nil
}
