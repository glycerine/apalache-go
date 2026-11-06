package lp

// Source: src/util/lp/lp_core_solver_base.hpp:390-444.

import "fmt"

// The source floating reduced-cost tests use a fixed, strict threshold,
// independently of the configurable primal and dual feasibility tolerances.
func (m *FloatCoreSolverBase) DIsNotNegative(j uint32) (bool, error) {
	if uint64(j) >= uint64(len(m.D)) {
		return false, ErrBounds
	}
	return m.D[j] > -0.00001, nil
}
func (m *FloatCoreSolverBase) DIsNotPositive(j uint32) (bool, error) {
	if uint64(j) >= uint64(len(m.D)) {
		return false, ErrBounds
	}
	return m.D[j] < 0.00001, nil
}

func (m *FloatCoreSolverBase) ColumnIsDualFeasible(j uint32) (bool, error) {
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed, ColumnBoxed, ColumnLowBound:
		if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.LowBounds)) {
			return false, ErrBounds
		}
		if m.AtBound((*m.X)[j], (*m.LowBounds)[j]) {
			ok, err := m.DIsNotNegative(j)
			if err != nil || ok {
				return ok, err
			}
		}
		if (*m.ColumnTypes)[j] == ColumnLowBound {
			return false, nil
		}
		if uint64(j) >= uint64(len(*m.UpperBounds)) {
			return false, ErrBounds
		}
		if !m.AtBound((*m.X)[j], (*m.UpperBounds)[j]) {
			return false, nil
		}
		return m.DIsNotPositive(j)
	case ColumnUpperBound:
		if m.Settings.DebugOut != nil {
			if _, err := fmt.Fprintln(m.Settings.DebugOut, "upper_bound type should be switched to low_bound"); err != nil {
				return false, err
			}
		}
		if err := sourceAssertionFailure(); err != nil {
			return false, err
		}
		fallthrough
	case ColumnFree:
		if uint64(j) >= uint64(len(m.D)) {
			return false, ErrBounds
		}
		return m.D[j] == 0, nil
	default:
		if m.Settings.DebugOut != nil {
			if _, err := fmt.Fprintf(m.Settings.DebugOut, "column = %d\n", j); err != nil {
				return false, err
			}
			name, err := ColumnTypeString((*m.ColumnTypes)[j])
			if err != nil {
				return false, err
			}
			if _, err := fmt.Fprintf(m.Settings.DebugOut, "unexpected column type = %s\n", name); err != nil {
				return false, err
			}
		}
		return false, sourceAssertionFailure()
	}
}

func (m *FloatCoreSolverBase) SetNonBasicXToCorrectBounds() error {
	for _, j := range *m.NBasis {
		if uint64(j) >= uint64(len(*m.ColumnTypes)) {
			return ErrBounds
		}
		var bounds *[]float64
		switch (*m.ColumnTypes)[j] {
		case ColumnBoxed:
			if uint64(j) >= uint64(len(m.D)) {
				return ErrBounds
			}
			bounds = m.LowBounds
			if m.D[j] < 0 {
				bounds = m.UpperBounds
			}
		case ColumnLowBound:
			bounds = m.LowBounds
		case ColumnUpperBound:
			bounds = m.UpperBounds
		default:
			continue
		}
		if uint64(j) >= uint64(len(*bounds)) || uint64(j) >= uint64(len(*m.X)) {
			return ErrBounds
		}
		(*m.X)[j] = (*bounds)[j]
		if sourceDebugEnabled && (*m.ColumnTypes)[j] != ColumnBoxed {
			ok, err := m.ColumnIsDualFeasible(j)
			if err != nil {
				return err
			}
			if !ok {
				return ErrAssertion
			}
		}
	}
	return nil
}
