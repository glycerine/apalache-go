package lp

// Source: src/util/lp/lp_primal_core_solver_tableau.h:350-383.

// Each arithmetic segment finishes its capture before delegated feasibility
// or cost operations, whose stopped numerical receipts remain their own.
func (m *PairCoreSolverBase) addTableauEnteringValue(entering uint32, delta *RationalPair) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return (*m.X)[entering].Add(m.manager, &(*m.X)[entering], delta)
}
func (m *PairCoreSolverBase) subtractTableauBasicValue(j uint32, delta *RationalPair, cell ColumnCell) (err error) {
	coefficient, e := m.A.ColumnValue(cell)
	if e != nil {
		return e
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return m.subtractCoreValueProduct(&(*m.X)[j], delta, coefficient)
}

// UpdateXTableau updates the entering value first. Each stored-column basic
// subtraction is immediately followed by membership/cost maintenance, before
// the next live delta read. Delta may alias an updated value.
func (m *PairPrimalCoreSolver) UpdateXTableau(entering uint32, delta *RationalPair) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if delta == nil || uint64(entering) >= uint64(len(*m.X)) {
		return ErrBounds
	}
	if err = m.addTableauEnteringValue(entering, delta); err != nil {
		return err
	}
	if m.UsingInfeasCosts && sourceDebugEnabled {
		feasible, e := m.ColumnIsFeasible(entering)
		if e != nil {
			return e
		}
		if !feasible {
			return ErrAssertion
		}
		if uint64(entering) >= uint64(len(*m.Costs)) {
			return ErrBounds
		}
		zero, e := m.manager.IsZero(&(*m.Costs)[entering])
		if e != nil {
			return e
		}
		if !zero {
			return ErrAssertion
		}
	}
	if uint64(entering) >= uint64(len(m.A.Columns)) {
		return ErrBounds
	}
	for _, cell := range m.A.Columns[entering] {
		if uint64(cell.Row) >= uint64(len(*m.Basis)) {
			return ErrBounds
		}
		j := (*m.Basis)[cell.Row]
		if uint64(j) >= uint64(len(*m.X)) {
			return ErrBounds
		}
		if err = m.subtractTableauBasicValue(j, delta, cell); err != nil {
			return err
		}
		if !m.UsingInfeasCosts {
			if err = m.UpdateColumnInInfSet((*m.Basis)[cell.Row]); err != nil {
				return err
			}
		} else {
			if err = m.UpdateInfCostForColumnTableau(j); err != nil {
				return err
			}
			if uint64(j) >= uint64(len(*m.Costs)) {
				return ErrBounds
			}
			zero, e := m.manager.IsZero(&(*m.Costs)[j])
			if e != nil {
				return e
			}
			if zero {
				m.InfSet.Erase(j)
			} else if err = m.InfSet.Insert(j); err != nil {
				return err
			}
		}
	}
	if sourceDebugEnabled {
		off, e := m.AMultXIsOff()
		if e != nil {
			return e
		}
		if off {
			return ErrAssertion
		}
	}
	return nil
}

// UpdateBasisAndXTableau preserves the source's ignored pivot bool: a false
// numerical pivot result still reaches ChangeBasis. Errors stop at their prefix.
func (m *PairPrimalCoreSolver) UpdateBasisAndXTableau(entering, leaving int32, delta *RationalPair) (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled && !m.Settings.UseTableau() {
		return false, ErrAssertion
	}
	en, lv := uint32(entering), uint32(leaving)
	if err = m.UpdateXTableau(en, delta); err != nil {
		return false, err
	}
	if uint64(lv) >= uint64(len(*m.Heading)) {
		return false, ErrBounds
	}
	if _, err = m.PivotColumnTableau(en, uint32((*m.Heading)[lv])); err != nil {
		return false, err
	}
	if err = m.ChangeBasis(en, lv); err != nil {
		return false, err
	}
	return true, nil
}
