package lp

// Source: src/util/lp/lp_core_solver_base.h:199-208.

// NeedToPivotToBasisTableau tests singleton columns and their unit values.
// Like the source it does not additionally require the cell row to equal i.
func (m *RationalCoreSolverBase) NeedToPivotToBasisTableau() (pivot bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled {
		ok, e := m.A.IsCorrect(nil)
		if e != nil {
			return false, e
		}
		if !ok {
			return false, ErrAssertion
		}
	}
	for i := uint32(0); i < m.A.RowCount(); i++ {
		if uint64(i) >= uint64(len(*m.Basis)) {
			return false, ErrBounds
		}
		j := (*m.Basis)[i]
		if uint64(j) >= uint64(len(m.A.Columns)) {
			return false, ErrBounds
		}
		column := m.A.Columns[j]
		if len(column) == 0 {
			if sourceDebugEnabled {
				return false, ErrAssertion
			}
			return false, ErrBounds
		}
		if len(column) > 1 {
			return true, nil
		}
		value, e := m.A.ColumnValue(column[0])
		if e != nil {
			return false, e
		}
		one, e := m.manager.IsOne(value)
		if e != nil {
			return false, e
		}
		if !one {
			return true, nil
		}
	}
	return false, nil
}
