package lp

// Source: src/util/lp/lp_primal_core_solver.hpp:59-148,233-271.

import "sort"

func (m *FloatPrimalCoreSolver) ColumnIsBeneficialForEnteringOnBreakpoints(j uint32) (bool, error) {
	if uint64(j) >= uint64(len(m.D)) || uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	d, eps := m.D[j], m.EpsilonOfReducedCost
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed:
		return false, nil
	case ColumnFree:
		return d > eps || d < -eps, nil
	case ColumnLowBound, ColumnUpperBound:
		upper := (*m.ColumnTypes)[j] == ColumnUpperBound
		if sourceDebugEnabled {
			ok, err := m.atColumnBound(j, upper)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, ErrAssertion
			}
		}
		if upper {
			return d > eps, nil
		}
		return d < -eps, nil
	case ColumnBoxed:
		low, err := m.atColumnBound(j, false)
		if err != nil {
			return false, err
		}
		if sourceDebugEnabled && !low {
			upper, err := m.atColumnBound(j, true)
			if err != nil {
				return false, err
			}
			if !upper {
				return false, ErrAssertion
			}
		}
		return (low && d < -eps) || (!low && d > eps), nil
	default:
		return false, sourceAssertionFailure()
	}
}

func (m *FloatPrimalCoreSolver) ColumnIsBeneficialForEnteringBasis(j uint32) (bool, error) {
	if m.UsingInfeasCosts && m.Settings.UseBreakpointsInFeasibilitySearch {
		return m.ColumnIsBeneficialForEnteringOnBreakpoints(j)
	}
	if uint64(j) >= uint64(len(m.D)) || uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	d, eps := m.D[j], m.EpsilonOfReducedCost
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed:
		return false, nil
	case ColumnFree:
		return d > eps || d < -eps, nil
	case ColumnLowBound:
		return d > eps, nil
	case ColumnUpperBound:
		return d < -eps, nil
	case ColumnBoxed:
		if d > eps || d < -eps {
			x, low, err := m.columnBound(j, false)
			if err != nil {
				return false, err
			}
			if uint64(j) >= uint64(len(*m.UpperBounds)) {
				return false, ErrBounds
			}
			upper := (*m.UpperBounds)[j]
			span := upper - low
			if d > eps {
				return x < upper-span/2, nil
			}
			return x > low+span/2, nil
		}
		return false, nil
	default:
		return false, sourceAssertionFailure()
	}
}

func (m *FloatPrimalCoreSolver) SortNonBasis() error {
	for _, j := range *m.NBasis {
		if uint64(j) >= uint64(len(m.D)) || uint64(j) >= uint64(len(m.ColumnNorms)) || uint64(j) >= uint64(len(m.SteepestEdge)) {
			return ErrBounds
		}
		d := m.D[j]
		m.SteepestEdge[j] = d * d / m.ColumnNorms[j]
	}
	// Use Go sorting; equal/unordered keys have no specified C++ sort order.
	sort.Slice(*m.NBasis, func(i, j int) bool { return m.SteepestEdge[(*m.NBasis)[i]] > m.SteepestEdge[(*m.NBasis)[j]] })
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

func (m *FloatPrimalCoreSolver) ChooseEnteringColumn(budget uint32) (int32, error) {
	if budget == 0 {
		return -1, nil
	}
	if m.BasisSortCounter == 0 {
		if err := m.SortNonBasis(); err != nil {
			return -1, err
		}
		m.BasisSortCounter = 20
	} else {
		m.BasisSortCounter--
	}
	best, position := 0.0, -1
	for i, j := range m.NonBasisList {
		if budget == 0 {
			break
		}
		ok, err := m.ColumnIsBeneficialForEnteringBasis(j)
		if err != nil {
			return -1, err
		}
		if !ok {
			continue
		}
		if uint64(j) >= uint64(len(m.ColumnNorms)) {
			return -1, ErrBounds
		}
		d := m.D[j]
		price := d * d / m.ColumnNorms[j]
		if price > best {
			best = price
			position = i
			budget--
		}
	}
	if position < 0 {
		return -1, nil
	}
	j := m.NonBasisList[position]
	m.SignOfEnteringDelta = -1
	if m.D[j] > 0 {
		m.SignOfEnteringDelta = 1
	}
	if m.UsingInfeasCosts && m.Settings.UseBreakpointsInFeasibilitySearch {
		m.SignOfEnteringDelta = -m.SignOfEnteringDelta
	}
	copy(m.NonBasisList[position:], m.NonBasisList[position+1:])
	m.NonBasisList[len(m.NonBasisList)-1] = j
	return int32(j), nil
}
