package lp

// Source: src/util/lp/lp_primal_core_solver_tableau.h:396-425.

// InitReducedCostsTableau uses the actual stored tableau columns. Cost
// switching completes before the ascending heading scan; basic costs and
// matrix coefficients remain live through each nonbasic compound subtraction.
func (m *RationalPrimalCoreSolver) InitReducedCostsTableau() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if m.CurrentXIsInfeasible() && !m.UsingInfeasCosts {
		if err = m.InitInfeasibilityCosts(); err != nil {
			return err
		}
	} else if m.CurrentXIsFeasible() && m.UsingInfeasCosts {
		if m.LookForFeasibleSolutionOnly {
			return nil
		}
		if err = m.assignCostValues(m.Costs, &m.CostsBackup.Data); err != nil {
			return err
		}
		m.UsingInfeasCosts = false
	}
	return m.fillReducedCostsTableau()
}

func (m *RationalPrimalCoreSolver) fillReducedCostsTableau() (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	size := len(*m.Heading)
	for j := 0; j < size; j++ {
		if j >= len(m.D.Data) {
			return ErrBounds
		}
		if (*m.Heading)[j] >= 0 {
			if err = m.manager.SetInt32(&m.D.Data[j], 0); err != nil {
				return err
			}
			continue
		}
		if j >= len(*m.Costs) {
			return ErrBounds
		}
		if err = m.manager.Copy(&m.D.Data[j], &(*m.Costs)[j]); err != nil {
			return err
		}
		if j >= len(m.A.Columns) {
			return ErrBounds
		}
		for _, cc := range m.A.Columns[j] {
			if uint64(cc.Row) >= uint64(len(*m.Basis)) {
				return ErrBounds
			}
			basic := (*m.Basis)[cc.Row]
			if uint64(basic) >= uint64(len(*m.Costs)) {
				return ErrBounds
			}
			coefficient, e := m.A.ColumnValue(cc)
			if e != nil {
				return e
			}
			m.work = NewIndexedRationalVector(m.manager, 2)
			if err = etaRationalProduct(m.manager, &m.work.Data[0], &m.work.Data[1], &(*m.Costs)[basic], coefficient); err != nil {
				return err
			}
			if err = m.manager.Sub(&m.D.Data[j], &m.work.Data[1], &m.D.Data[j]); err != nil {
				return err
			}
			if err = m.work.Close(); err != nil {
				return err
			}
			m.work = nil
		}
	}
	return nil
}
