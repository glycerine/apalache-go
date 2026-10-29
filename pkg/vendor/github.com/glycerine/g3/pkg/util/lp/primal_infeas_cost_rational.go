package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:1103-1118,1168-1225;
// lp_primal_core_solver.h:403-408,783-789.

func (m *RationalPrimalCoreSolver) NeedToSwitchCosts() bool {
	if m.Settings.SimplexStrategy == StrategyTableauRows {
		return false
	}
	return m.CurrentXIsFeasible() == m.UsingInfeasCosts
}

func (m *RationalPrimalCoreSolver) setInfeasibilityCost(j uint32, cost int32) (err error) {
	if uint64(j) >= uint64(len(*m.Costs)) {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return m.manager.SetInt32(&(*m.Costs)[j], cost)
}
func (m *RationalPrimalCoreSolver) negateInfeasibilityCost(j uint32) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 1)
	temp := &m.work.Data[0]
	if err = m.manager.Copy(temp, &(*m.Costs)[j]); err != nil {
		return err
	}
	if err = m.manager.Neg(temp); err != nil {
		return err
	}
	if err = m.manager.Copy(&(*m.Costs)[j], temp); err != nil {
		return err
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}

// InitInfeasibilityCostForColumn assigns the raw infeasibility cost,
// updates membership, then optionally negates it. Comparisons stay with the base.
func (m *RationalPrimalCoreSolver) InitInfeasibilityCostForColumn(j uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.initInfeasibilityCostForColumnValue(j)
}
func (m *RationalPrimalCoreSolver) initInfeasibilityCostForColumnValue(j uint32) (err error) {
	if uint64(j) >= uint64(len(*m.Heading)) {
		return ErrBounds
	}
	if (*m.Heading)[j] < 0 {
		if err = m.setInfeasibilityCost(j, 0); err != nil {
			return err
		}
		m.InfSet.Erase(j)
		return nil
	}
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return ErrBounds
	}
	kind := (*m.ColumnTypes)[j]
	var cost int32
	write := true
	switch kind {
	case ColumnFixed, ColumnBoxed, ColumnLowBound, ColumnUpperBound:
		if uint64(j) >= uint64(len(*m.X)) {
			return ErrBounds
		}
		x := &(*m.X)[j]
		above := false
		if kind != ColumnLowBound {
			if uint64(j) >= uint64(len(*m.UpperBounds)) {
				return ErrBounds
			}
			above, err = m.AboveBound(x, &(*m.UpperBounds)[j])
			if err != nil {
				return err
			}
		}
		if above {
			cost = 1
		} else if kind != ColumnUpperBound {
			if uint64(j) >= uint64(len(*m.LowBounds)) {
				return ErrBounds
			}
			below, e := m.BelowBound(x, &(*m.LowBounds)[j])
			if e != nil {
				return e
			}
			if below {
				cost = -1
			}
		}
	case ColumnFree:
	default:
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
		// The native source default leaves its old cost intact.
		write = false
	}
	if write {
		if err = m.setInfeasibilityCost(j, cost); err != nil {
			return err
		}
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
	} else {
		if err = m.InfSet.Insert(j); err != nil {
			return err
		}
	}
	if m.Settings == nil {
		return ErrBounds
	}
	if !m.Settings.UseBreakpointsInFeasibilitySearch {
		return m.negateInfeasibilityCost(j)
	}
	return nil
}

func (m *RationalPrimalCoreSolver) InitInfeasibilityCosts() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	n := m.A.ColumnCount()
	if sourceDebugEnabled && (uint64(len(*m.X)) < uint64(n) || uint64(len(*m.ColumnTypes)) < uint64(n)) {
		return ErrAssertion
	}
	for j := n; j > 0; {
		j--
		if err = m.initInfeasibilityCostForColumnValue(j); err != nil {
			return err
		}
	}
	m.UsingInfeasCosts = true
	return nil
}

func (m *RationalPrimalCoreSolver) InitInfeasibilityCostsForChangedBasisOnly() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.initInfeasibilityCostsForChangedBasisOnlyValue()
}
func (m *RationalPrimalCoreSolver) initInfeasibilityCostsForChangedBasisOnlyValue() (err error) {
	for _, i := range m.Ed.Index {
		if uint64(i) >= uint64(len(*m.Basis)) {
			return ErrBounds
		}
		if err = m.initInfeasibilityCostForColumnValue((*m.Basis)[i]); err != nil {
			return err
		}
	}
	m.UsingInfeasCosts = true
	return nil
}

func (m *RationalPrimalCoreSolver) InitInfeasibilityAfterUpdateXIfInf(leaving uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if !m.UsingInfeasCosts {
		return nil
	}
	if err = m.initInfeasibilityCostsForChangedBasisOnlyValue(); err != nil {
		return err
	}
	if err = m.setInfeasibilityCost(leaving, 0); err != nil {
		return err
	}
	m.InfSet.Erase(leaving)
	return nil
}
