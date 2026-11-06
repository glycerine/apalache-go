package lp

// Source: src/util/lp/lp_primal_core_solver.h:403-409;
// lp_primal_core_solver.hpp:574-627,666-674,996-1046.

import "math"

func (m *FloatPrimalCoreSolver) NeedToSwitchCosts() bool {
	if m.Settings.SimplexStrategy == StrategyTableauRows {
		return false
	}
	return m.CurrentXIsFeasible() == m.UsingInfeasCosts
}

func (m *FloatPrimalCoreSolver) UpdateReducedCostsFromPivotRow(entering, leaving uint32) error {
	if sourceDebugEnabled {
		if uint64(entering) >= uint64(len(*m.Heading)) || uint64(leaving) >= uint64(len(*m.Heading)) {
			return ErrBounds
		}
		if (*m.Heading)[entering] < 0 || uint32((*m.Heading)[entering]) >= m.A.RowCount() || (*m.Heading)[leaving] >= 0 {
			return ErrAssertion
		}
	}
	if uint64(entering) >= uint64(len(m.PivotRow.Data)) || uint64(entering) >= uint64(len(m.D)) {
		return ErrBounds
	}
	pivot := m.PivotRow.Data[entering]
	dq := m.D[entering] / pivot
	for _, j := range m.PivotRow.Index {
		if uint64(j) >= uint64(len(*m.Heading)) {
			return ErrBounds
		}
		if (*m.Heading)[j] >= 0 {
			continue
		}
		if j != leaving {
			if uint64(j) >= uint64(len(m.D)) || uint64(j) >= uint64(len(m.PivotRow.Data)) {
				return ErrBounds
			}
			m.D[j] -= dq * m.PivotRow.Data[j]
		}
	}
	if uint64(leaving) >= uint64(len(m.D)) {
		return ErrBounds
	}
	m.D[leaving] = -dq
	if m.CurrentXIsInfeasible() && !m.Settings.UseBreakpointsInFeasibilitySearch {
		if uint64(leaving) >= uint64(len(*m.Costs)) {
			return ErrBounds
		}
		m.D[leaving] -= (*m.Costs)[leaving]
		(*m.Costs)[leaving] = 0
	}
	m.D[entering] = 0
	return nil
}

// RefreshReducedCostAtEnteringAndCheckThatItIsOff returns the source code:
// 0 close enough, 1 substantial drift, 2 no longer profitable. It publishes
// the refreshed cost before classifying drift and profitability.
func (m *FloatPrimalCoreSolver) RefreshReducedCostAtEnteringAndCheckThatItIsOff(entering uint32) (int, error) {
	if uint64(entering) >= uint64(len(m.D)) {
		return 0, ErrBounds
	}
	old := m.D[entering]
	if sourceDebugEnabled && !(math.Abs(old) > m.EpsilonOfReducedCost) {
		return 0, ErrAssertion
	}
	if uint64(entering) >= uint64(len(*m.Costs)) {
		return 0, ErrBounds
	}
	refreshed := (*m.Costs)[entering]
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(*m.Basis)) || uint64(i) >= uint64(len(m.Ed.Data)) {
			return 0, ErrBounds
		}
		j := (*m.Basis)[i]
		if uint64(j) >= uint64(len(*m.Costs)) {
			return 0, ErrBounds
		}
		refreshed -= (*m.Costs)[j] * m.Ed.Data[i]
	}
	m.D[entering] = refreshed
	delta := math.Abs(old - refreshed)
	if old > m.EpsilonOfReducedCost {
		if refreshed <= 0 {
			return 2, nil
		}
	} else if refreshed > -m.EpsilonOfReducedCost {
		return 2, nil
	}
	if delta*2 > math.Abs(old) {
		return 1, nil
	}
	return 0, nil
}

func (m *FloatPrimalCoreSolver) CalcWorkingVectorBetaForColumnNorms() error {
	if err := m.Beta.Assign(m.Ed); err != nil {
		return err
	}
	if m.Factorization == nil {
		return ErrBounds
	}
	return m.Factorization.SolveYBWithErrorCheckIndexed(m.Beta, *m.Heading, *m.Basis, m.Settings)
}

func (m *FloatPrimalCoreSolver) CalculateNormOfEnteringExactly() (float64, error) {
	r := 1.0
	for _, i := range m.Ed.Index {
		if uint64(i) >= uint64(len(m.Ed.Data)) {
			return r, ErrBounds
		}
		t := m.Ed.Data[i]
		r += t * t
	}
	return r, nil
}

func (m *FloatPrimalCoreSolver) UpdateOrInitColumnNorms(entering, leaving uint32) error {
	frequency := m.Settings.ColumnNormsUpdateFrequency
	if sourceDebugEnabled && m.ColumnNormUpdateCounter > frequency {
		return ErrAssertion
	}
	if m.ColumnNormUpdateCounter == frequency {
		m.ColumnNormUpdateCounter = 0
		return m.InitColumnNorms()
	}
	m.ColumnNormUpdateCounter++
	return m.UpdateColumnNorms(entering, leaving)
}

func (m *FloatPrimalCoreSolver) UpdateColumnNorms(entering, leaving uint32) error {
	if uint64(entering) >= uint64(len(m.PivotRow.Data)) {
		return ErrBounds
	}
	pivot := m.PivotRow.Data[entering]
	norm, err := m.CalculateNormOfEnteringExactly()
	if err != nil {
		return err
	}
	g := norm / pivot / pivot
	if g < 0.000001 {
		g = 0.000001
	}
	if uint64(leaving) >= uint64(len(m.ColumnNorms)) {
		return ErrBounds
	}
	m.ColumnNorms[leaving] = g
	for _, j := range m.PivotRow.Index {
		if j == leaving {
			continue
		}
		if uint64(j) >= uint64(len(m.PivotRow.Data)) {
			return ErrBounds
		}
		t := m.PivotRow.Data[j]
		s, err := m.A.DotColumn(m.Beta.Data, j)
		if err != nil {
			return err
		}
		k := -2 / pivot
		tp := t / pivot
		if uint64(j) >= uint64(len(*m.ColumnTypes)) {
			return ErrBounds
		}
		if (*m.ColumnTypes)[j] != ColumnFixed {
			if uint64(j) >= uint64(len(m.ColumnNorms)) {
				return ErrBounds
			}
			candidate := m.ColumnNorms[j] + t*(t*g+k*s)
			lower := 1 + tp*tp
			// std::max retains its first operand on unordered comparisons.
			if candidate < lower {
				candidate = lower
			}
			m.ColumnNorms[j] = candidate
		}
	}
	return nil
}
