package lp

// Source: src/util/lp/lp_primal_core_solver_tableau.h:51-94.

// ChooseEnteringColumnTableau prices actual tableau columns. Only strict
// sparsity improvements consume budget; ties consume RNG before rotation.
func (m *RationalPrimalCoreSolver) ChooseEnteringColumnTableau() (entering int32, err error) {
	if err = m.ready(); err != nil {
		return -1, err
	}
	defer func() { m.stopped = err != nil }()
	budget, e := m.NumberOfNonBasicColumnsToTryForEnter()
	if e != nil {
		return -1, e
	}
	if budget == 0 {
		return -1, nil
	}
	if m.BasisSortCounter == 0 {
		if err = m.SortNonBasis(); err != nil {
			return -1, err
		}
		m.BasisSortCounter = 20
	} else {
		m.BasisSortCounter--
	}
	best, position := m.A.RowCount()+1, -1
	for i, j := range m.NonBasisList {
		if budget == 0 {
			break
		}
		beneficial, e := m.ColumnIsBeneficialForEnteringBasis(j)
		if e != nil {
			return -1, e
		}
		if !beneficial {
			continue
		}
		if uint64(j) >= uint64(len(m.A.Columns)) {
			return -1, ErrBounds
		}
		count := uint32(len(m.A.Columns[j]))
		if count < best {
			best, position = count, i
			budget--
		} else if count == best && m.Settings.RandomNext()%2 == 0 {
			position = i
		}
	}
	if position < 0 {
		return -1, nil
	}
	j := m.NonBasisList[position]
	if uint64(j) >= uint64(len(m.D.Data)) {
		return -1, ErrBounds
	}
	positive, e := m.manager.IsPos(&m.D.Data[j])
	if e != nil {
		return -1, e
	}
	m.SignOfEnteringDelta = -1
	if positive {
		m.SignOfEnteringDelta = 1
	}
	if m.UsingInfeasCosts && m.Settings.UseBreakpointsInFeasibilitySearch {
		m.SignOfEnteringDelta = -m.SignOfEnteringDelta
	}
	copy(m.NonBasisList[position:], m.NonBasisList[position+1:])
	m.NonBasisList[len(m.NonBasisList)-1] = j
	return int32(j), nil
}
