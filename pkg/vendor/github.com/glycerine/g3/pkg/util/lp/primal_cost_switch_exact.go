package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:630-639,1292-1304.

import "github.com/glycerine/g3/pkg/util/numeral"

// assignCostValues follows the live dense cost assignment: destroy old values
// in ascending order, publish new storage, then copy source values in order.
// This does not use indexed-vector Assign, which copies only sparse entries.
func (m *PairPrimalCoreSolver) assignCostValues(dst, src *[]numeral.Rational) (err error) {
	if dst == src {
		return nil
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	for i := range *dst {
		if err = m.manager.Del(&(*dst)[i]); err != nil {
			return err
		}
		(*dst)[i] = numeral.NewRational()
	}
	*dst = nil
	*dst = make([]numeral.Rational, len(*src))
	for i := range *dst {
		(*dst)[i] = numeral.NewRational()
	}
	for i := range *src {
		if err = m.manager.Copy(&(*dst)[i], &(*src)[i]); err != nil {
			return err
		}
	}
	return nil
}

// BackupAndNormalizeCosts only backs up costs for the precise specialization.
func (m *PairPrimalCoreSolver) BackupAndNormalizeCosts() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if m.LookForFeasibleSolutionOnly {
		return nil
	}
	return m.assignCostValues(&m.CostsBackup.Data, m.Costs)
}

func (m *PairPrimalCoreSolver) InitReducedCosts() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled && m.Settings.UseTableau() {
		return ErrAssertion
	}
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
	return m.InitReducedCostsForOneIteration()
}
