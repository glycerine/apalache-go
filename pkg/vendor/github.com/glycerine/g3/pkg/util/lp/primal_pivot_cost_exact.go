package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:574-600.

import "github.com/glycerine/g3/pkg/util/numeral"

// UpdateReducedCostsFromPivotRow uses the old pivot row after the headings
// have changed. It retains local pivot/dq values while row and cost references
// stay live; source member writes are published in their original order.
func (m *PairPrimalCoreSolver) UpdateReducedCostsFromPivotRow(entering, leaving uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled {
		if uint64(entering) >= uint64(len(*m.Heading)) {
			return ErrBounds
		}
		en := (*m.Heading)[entering]
		if en < 0 || uint64(en) >= uint64(m.A.RowCount()) {
			return ErrAssertion
		}
		if uint64(leaving) >= uint64(len(*m.Heading)) {
			return ErrBounds
		}
		if (*m.Heading)[leaving] >= 0 {
			return ErrAssertion
		}
	}
	if uint64(entering) >= uint64(len(m.PivotRow.Data)) {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 5)
	pivot, tmp, dq, product, negative := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2], &m.work.Data[3], &m.work.Data[4]
	if err = m.manager.Copy(pivot, &m.PivotRow.Data[entering]); err != nil {
		return err
	}
	if uint64(entering) >= uint64(len(m.D.Data)) {
		return ErrBounds
	}
	if err = etaRationalQuotient(m.manager, tmp, dq, &m.D.Data[entering], pivot); err != nil {
		return err
	}
	for _, j := range m.PivotRow.Index {
		if uint64(j) >= uint64(len(*m.Heading)) {
			return ErrBounds
		}
		if (*m.Heading)[j] >= 0 {
			continue
		}
		if j == leaving {
			continue
		}
		if uint64(j) >= uint64(len(m.D.Data)) || uint64(j) >= uint64(len(m.PivotRow.Data)) {
			return ErrBounds
		}
		if err = etaRationalProduct(m.manager, tmp, product, dq, &m.PivotRow.Data[j]); err != nil {
			return err
		}
		if err = m.manager.Sub(&m.D.Data[j], product, &m.D.Data[j]); err != nil {
			return err
		}
		if err = m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
	}
	if uint64(leaving) >= uint64(len(m.D.Data)) {
		return ErrBounds
	}
	if err = m.manager.Copy(negative, dq); err != nil {
		return err
	}
	if err = m.manager.Neg(negative); err != nil {
		return err
	}
	if err = m.manager.Copy(&m.D.Data[leaving], negative); err != nil {
		return err
	}
	if err = m.manager.Del(negative); err != nil {
		return err
	}
	*negative = numeral.NewRational()
	if m.CurrentXIsInfeasible() && !m.Settings.UseBreakpointsInFeasibilitySearch {
		if uint64(leaving) >= uint64(len(*m.Costs)) {
			return ErrBounds
		}
		if err = m.manager.Sub(&m.D.Data[leaving], &(*m.Costs)[leaving], &m.D.Data[leaving]); err != nil {
			return err
		}
		if err = m.manager.SetInt32(&(*m.Costs)[leaving], 0); err != nil {
			return err
		}
	}
	if err = m.manager.SetInt32(&m.D.Data[entering], 0); err != nil {
		return err
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}
