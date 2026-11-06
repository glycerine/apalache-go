package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:31-57,86-110,151-232.

import "sort"

// pricingCompareEpsilon owns only its rational expression/comparison. Bound
// comparisons are delegated separately to the exact base core.
func (m *PairPrimalCoreSolver) pricingCompareEpsilon(j uint32, negative bool) (value int, err error) {
	if uint64(j) >= uint64(len(m.D.Data)) {
		return 0, ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return 0, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	epsilon := &m.EpsilonOfReducedCost
	if negative {
		m.work = NewIndexedRationalVector(m.manager, 1)
		epsilon = &m.work.Data[0]
		if err = m.manager.Copy(epsilon, &m.EpsilonOfReducedCost); err != nil {
			return 0, err
		}
		if err = m.manager.Neg(epsilon); err != nil {
			return 0, err
		}
	}
	value, err = m.manager.Compare(&m.D.Data[j], epsilon)
	if err != nil {
		return 0, err
	}
	if m.work != nil {
		if err = m.work.Close(); err != nil {
			return 0, err
		}
		m.work = nil
	}
	return value, nil
}

func (m *PairPrimalCoreSolver) pricingAtBound(j uint32, upper bool) (bool, error) {
	if uint64(j) >= uint64(len(*m.X)) {
		return false, ErrBounds
	}
	bounds := m.LowBounds
	if upper {
		bounds = m.UpperBounds
	}
	if uint64(j) >= uint64(len(*bounds)) {
		return false, ErrBounds
	}
	return m.AtBound(&(*m.X)[j], &(*bounds)[j])
}

func (m *PairPrimalCoreSolver) ColumnIsBeneficialForEnteringOnBreakpoints(j uint32) (beneficial bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if uint64(j) >= uint64(len(m.D.Data)) || uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed:
		return false, nil
	case ColumnLowBound, ColumnUpperBound:
		upper := (*m.ColumnTypes)[j] == ColumnUpperBound
		if sourceDebugEnabled {
			at, e := m.pricingAtBound(j, upper)
			if e != nil {
				return false, e
			}
			if !at {
				return false, ErrAssertion
			}
		}
		cmp, e := m.pricingCompareEpsilon(j, !upper)
		if e != nil {
			return false, e
		}
		if upper {
			return cmp > 0, nil
		}
		return cmp < 0, nil
	case ColumnBoxed:
		low, e := m.pricingAtBound(j, false)
		if e != nil {
			return false, e
		}
		if sourceDebugEnabled && !low {
			upper, e := m.pricingAtBound(j, true)
			if e != nil {
				return false, e
			}
			if !upper {
				return false, ErrAssertion
			}
		}
		cmp, e := m.pricingCompareEpsilon(j, low)
		if e != nil {
			return false, e
		}
		if low {
			return cmp < 0, nil
		}
		return cmp > 0, nil
	case ColumnFree:
		cmp, e := m.pricingCompareEpsilon(j, false)
		if e != nil {
			return false, e
		}
		if cmp > 0 {
			return true, nil
		}
		cmp, e = m.pricingCompareEpsilon(j, true)
		return cmp < 0, e
	default:
		return false, sourceAssertionFailure()
	}
}

func (m *PairPrimalCoreSolver) ColumnIsBeneficialForEnteringBasis(j uint32) (beneficial bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if m.UsingInfeasCosts && m.Settings.UseBreakpointsInFeasibilitySearch {
		return m.ColumnIsBeneficialForEnteringOnBreakpoints(j)
	}
	if uint64(j) >= uint64(len(m.D.Data)) || uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	kind := (*m.ColumnTypes)[j]
	if kind == ColumnFixed {
		return false, nil
	}
	if kind != ColumnFree && kind != ColumnLowBound && kind != ColumnUpperBound && kind != ColumnBoxed {
		return false, sourceAssertionFailure()
	}
	sign, e := m.manager.Sign(&m.D.Data[j])
	if e != nil {
		return false, e
	}
	switch kind {
	case ColumnFree:
		return sign != 0, nil
	case ColumnLowBound:
		if sign > 0 {
			return true, nil
		}
		if sign == 0 {
			return false, nil
		}
	case ColumnUpperBound:
		if sign < 0 {
			return true, nil
		}
		if sign == 0 {
			return false, nil
		}
	case ColumnBoxed:
		if sign == 0 {
			return false, nil
		}
	}
	if uint64(j) >= uint64(len(*m.X)) {
		return false, ErrBounds
	}
	if sign > 0 {
		if uint64(j) >= uint64(len(*m.UpperBounds)) {
			return false, ErrBounds
		}
		return m.BelowBound(&(*m.X)[j], &(*m.UpperBounds)[j])
	}
	if uint64(j) >= uint64(len(*m.LowBounds)) {
		return false, ErrBounds
	}
	return m.AboveBound(&(*m.X)[j], &(*m.LowBounds)[j])
}

// SortNonBasis uses Go sorting with the literal source comparison. Zero-count
// columns compare neither before nor after nonzero columns; their relative
// placement is not a portable C++ sort guarantee.
func (m *PairPrimalCoreSolver) SortNonBasis() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	tableau := m.Settings.UseTableau()
	for _, j := range *m.NBasis {
		if tableau {
			if uint64(j) >= uint64(len(m.A.Columns)) {
				return ErrBounds
			}
		} else if uint64(j) >= uint64(len(m.ColumnsNZ)) {
			return ErrBounds
		}
	}
	sort.Slice(*m.NBasis, func(i, j int) bool {
		a, b := (*m.NBasis)[i], (*m.NBasis)[j]
		var ca, cb uint32
		if tableau {
			ca, cb = uint32(len(m.A.Columns[a])), uint32(len(m.A.Columns[b]))
		} else {
			ca, cb = m.ColumnsNZ[a], m.ColumnsNZ[b]
		}
		if ca == 0 && cb != 0 {
			return false
		}
		return ca < cb
	})
	m.NonBasisList = m.NonBasisList[:0]
	for i, j := range *m.NBasis {
		if uint64(j) >= uint64(len(*m.Heading)) {
			return ErrBounds
		}
		(*m.Heading)[j] = -int32(i) - 1
		m.NonBasisList = append(m.NonBasisList, j)
	}
	return nil
}

// ChooseEnteringColumn implements the precise minimum-sparsity pricing path.
// The budget counts strict improvements, while equal sparsity consumes RNG
// without spending budget. The selected column rotates to the list's end.
func (m *PairPrimalCoreSolver) ChooseEnteringColumn(budget uint32) (entering int32, err error) {
	if err = m.ready(); err != nil {
		return -1, err
	}
	defer func() { m.stopped = err != nil }()
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
		if uint64(j) >= uint64(len(m.ColumnsNZ)) {
			return -1, ErrBounds
		}
		count := m.ColumnsNZ[j]
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
