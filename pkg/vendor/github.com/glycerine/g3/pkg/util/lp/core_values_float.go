package lp

// Source: src/util/lp/lp_core_solver_base.hpp:225-295,334-348,460-477,
// 758-766,790-798,812-851. Retained double/double value reconstruction.

import "math"

func (m *FloatCoreSolverBase) RsMinusAnX(rs []float64) error {
	for row := m.A.RowCount(); row > 0; {
		row--
		if uint64(row) >= uint64(len(rs)) || uint64(row) >= uint64(len(*m.B)) {
			return ErrBounds
		}
		rs[row] = (*m.B)[row]
		for _, c := range m.A.Rows[row] {
			if uint64(c.Column) >= uint64(len(*m.Heading)) {
				return ErrBounds
			}
			if (*m.Heading)[c.Column] < 0 {
				if uint64(c.Column) >= uint64(len(*m.X)) {
					return ErrBounds
				}
				rs[row] -= (*m.X)[c.Column] * c.Value
			}
		}
	}
	return nil
}

func (m *FloatCoreSolverBase) CopyRsToXB(rs []float64) error {
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(*m.Basis)) {
			return ErrBounds
		}
		j := (*m.Basis)[i]
		if uint64(j) >= uint64(len(*m.X)) || uint64(i) >= uint64(len(rs)) {
			return ErrBounds
		}
		(*m.X)[j] = rs[i]
	}
	return nil
}

func (m *FloatCoreSolverBase) FindErrorInBxB(rs []float64) error {
	for row := m.A.RowCount(); row > 0; {
		row--
		if uint64(row) >= uint64(len(rs)) {
			return ErrBounds
		}
		for _, c := range m.A.Rows[row] {
			if uint64(c.Column) >= uint64(len(*m.Heading)) {
				return ErrBounds
			}
			if (*m.Heading)[c.Column] >= 0 {
				if uint64(c.Column) >= uint64(len(*m.X)) {
					return ErrBounds
				}
				rs[row] -= (*m.X)[c.Column] * c.Value
			}
		}
	}
	return nil
}

func (m *FloatCoreSolverBase) AddDeltaToXB(delta []float64) error {
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(*m.Basis)) {
			return ErrBounds
		}
		j := (*m.Basis)[i]
		if uint64(j) >= uint64(len(*m.X)) || uint64(i) >= uint64(len(delta)) {
			return ErrBounds
		}
		// Despite its name, the retained source subtracts this correction.
		(*m.X)[j] -= delta[i]
	}
	return nil
}

func (m *FloatCoreSolverBase) SolveAxEqualsB() error {
	rs := make([]float64, m.A.RowCount())
	if err := m.RsMinusAnX(rs); err != nil {
		return err
	}
	rrs := append([]float64(nil), rs...)
	if m.Factorization == nil {
		return ErrBounds
	}
	if err := m.Factorization.SolveBy(rs); err != nil {
		return err
	}
	if err := m.CopyRsToXB(rs); err != nil {
		return err
	}
	if err := m.FindErrorInBxB(rrs); err != nil {
		return err
	}
	if err := m.Factorization.SolveBy(rrs); err != nil {
		return err
	}
	return m.AddDeltaToXB(rrs)
}

func (m *FloatCoreSolverBase) rowResidualIsOff(i uint32) (bool, error) {
	if uint64(i) >= uint64(len(*m.B)) {
		return false, ErrBounds
	}
	dot, err := m.A.DotRow(i, *m.X)
	if err != nil {
		return false, err
	}
	b := (*m.B)[i]
	return math.Abs(b-dot) > m.Settings.RefactorTolerance*(1+0.1*math.Abs(b)), nil
}
func (m *FloatCoreSolverBase) AMultXIsOff() (bool, error) {
	if sourceDebugEnabled && len(*m.X) != int(m.A.ColumnCount()) {
		return false, ErrAssertion
	}
	for i := uint32(0); i < m.A.RowCount(); i++ {
		bad, err := m.rowResidualIsOff(i)
		if err != nil || bad {
			return bad, err
		}
	}
	return false, nil
}
func (m *FloatCoreSolverBase) AMultXIsOffOnIndex(index []uint32) (bool, error) {
	if sourceDebugEnabled && len(*m.X) != int(m.A.ColumnCount()) {
		return false, ErrAssertion
	}
	for _, i := range index {
		bad, err := m.rowResidualIsOff(i)
		if err != nil || bad {
			return bad, err
		}
	}
	return false, nil
}
func (m *FloatCoreSolverBase) FindXBySolving() (bool, error) {
	if err := m.SolveAxEqualsB(); err != nil {
		return false, err
	}
	bad, err := m.AMultXIsOff()
	return !bad, err
}

func (m *FloatCoreSolverBase) UpdateX(entering uint32, delta float64) error {
	if uint64(entering) >= uint64(len(*m.X)) {
		return ErrBounds
	}
	(*m.X)[entering] += delta
	if !m.Settings.UseTableau() {
		for _, i := range m.Ed.Index {
			if uint64(i) >= uint64(len(*m.Basis)) {
				return ErrBounds
			}
			j := (*m.Basis)[i]
			if uint64(j) >= uint64(len(*m.X)) || uint64(i) >= uint64(len(m.CopyXB)) {
				return ErrBounds
			}
			m.CopyXB[i] = (*m.X)[j]
			if uint64(i) >= uint64(len(m.Ed.Data)) {
				return ErrBounds
			}
			(*m.X)[j] -= delta * m.Ed.Data[i]
		}
	} else {
		if uint64(entering) >= uint64(len(m.A.Columns)) {
			return ErrBounds
		}
		for _, c := range m.A.Columns[entering] {
			i := c.Row
			if uint64(i) >= uint64(len(*m.Basis)) {
				return ErrBounds
			}
			j := (*m.Basis)[i]
			if uint64(j) >= uint64(len(*m.X)) {
				return ErrBounds
			}
			v, err := m.A.ColumnValue(c)
			if err != nil {
				return err
			}
			(*m.X)[j] -= delta * v
		}
	}
	return nil
}
func (m *FloatCoreSolverBase) RestoreX(entering uint32, t float64) error {
	if t == 0 {
		return nil
	}
	if uint64(entering) >= uint64(len(*m.X)) {
		return ErrBounds
	}
	(*m.X)[entering] -= t
	for _, i := range m.Ed.Index {
		if uint64(i) >= uint64(len(*m.Basis)) {
			return ErrBounds
		}
		j := (*m.Basis)[i]
		if uint64(j) >= uint64(len(*m.X)) || uint64(i) >= uint64(len(m.CopyXB)) {
			return ErrBounds
		}
		(*m.X)[j] = m.CopyXB[i]
	}
	return nil
}
