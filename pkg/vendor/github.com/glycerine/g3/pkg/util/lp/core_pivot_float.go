package lp

// Source: src/util/lp/lp_core_solver_base.hpp:300-330, double/double.

func (m *FloatCoreSolverBase) CalculatePivotRowOfBInverse(row uint32) error {
	if sourceDebugEnabled && m.Settings.UseTableau() {
		return ErrAssertion
	}
	// indexed_vector::is_OK is a true stub in the retained source.
	if err := m.PivotRowB1.Clear(); err != nil {
		return err
	}
	if err := m.PivotRowB1.SetValue(1, row); err != nil {
		return err
	}
	if m.Factorization == nil {
		return ErrBounds
	}
	return m.Factorization.SolveYBWithErrorCheckIndexed(m.PivotRowB1, *m.Heading, *m.Basis, m.Settings)
}

func (m *FloatCoreSolverBase) CalculatePivotRowWhenInverseReady(row uint32) error {
	if err := m.PivotRow.Clear(); err != nil {
		return err
	}
	for _, i := range m.PivotRowB1.Index {
		if uint64(i) >= uint64(len(m.PivotRowB1.Data)) {
			return ErrBounds
		}
		v := m.PivotRowB1.Data[i]
		if v == 0 {
			continue
		}
		if uint64(i) >= uint64(len(m.A.Rows)) {
			return ErrBounds
		}
		for _, c := range m.A.Rows[i] {
			if uint64(c.Column) >= uint64(len(*m.Heading)) {
				return ErrBounds
			}
			if (*m.Heading)[c.Column] < 0 {
				if err := m.PivotRow.AddWithDropTolerance(c.Column, c.Value*v); err != nil {
					return err
				}
			}
		}
	}
	// The source only uses row to update RowsNZ for precise instantiations.
	return nil
}
