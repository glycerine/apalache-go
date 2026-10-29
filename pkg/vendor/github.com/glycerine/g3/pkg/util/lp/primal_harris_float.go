package lp

// Source: src/util/lp/lp_primal_core_solver.h:328-374,539-742;
// lp_primal_core_solver.hpp:305-392. Retained double/double Harris path.

import "math"

func (m *FloatPrimalCoreSolver) RestoreHarrisEps() {
	m.ConvertedHarrisEpsilon = m.Settings.HarrisFeasibilityTolerance
}
func (m *FloatPrimalCoreSolver) ZeroHarrisEps() { m.ConvertedHarrisEpsilon = 0 }
func (m *FloatPrimalCoreSolver) HarrisEpsForBound(b float64) float64 {
	return (1 + math.Abs(b)/10) * m.ConvertedHarrisEpsilon / 3
}

// LimitTheta preserves std::min's first-operand choice, including NaNs and ties.
func (m *FloatPrimalCoreSolver) LimitTheta(lim float64, theta *float64, unlimited *bool) {
	if *unlimited || !(*theta < lim) {
		*theta = lim
	}
	*unlimited = false
}

func (m *FloatPrimalCoreSolver) harrisOneBound(j uint32, d float64, upper, feasible bool, theta *float64, unlimited *bool) error {
	positive := d > 0
	// The source selects the negative helper on zero/NaN upper movement,
	// and the positive helper on zero/NaN lower movement.
	if !feasible {
		positive = upper && d > 0 || !upper && !(d < 0)
	}
	if sourceDebugEnabled && (positive && !(d > 0) || !positive && !(d < 0)) {
		return ErrAssertion
	}
	x, b, err := m.columnBound(j, upper)
	if err != nil {
		return err
	}
	eps := m.HarrisEpsForBound(b)
	if feasible {
		if positive {
			m.LimitTheta((b-x+eps)/d, theta, unlimited)
		} else {
			m.LimitTheta((b-x-eps)/d, theta, unlimited)
		}
	} else if positive {
		if m.BelowBound(x, b) {
			m.LimitTheta((b-x+eps)/d, theta, unlimited)
		} else if upper && !m.AboveBound(x, b) {
			*theta = 0
			*unlimited = false
		}
	} else {
		if m.AboveBound(x, b) {
			m.LimitTheta((b-x-eps)/d, theta, unlimited)
		} else if !upper && !m.BelowBound(x, b) {
			*theta = 0
			*unlimited = false
		}
	}
	return nil
}

func (m *FloatPrimalCoreSolver) harrisBoxed(j uint32, d float64, theta *float64, unlimited *bool) error {
	if d > 0 {
		x, low, err := m.columnBound(j, false)
		if err != nil {
			return err
		}
		_, upper, err := m.columnBound(j, true)
		if err != nil {
			return err
		}
		if m.BelowBound(x, low) {
			// Source deliberately scales this low crossing using the upper bound.
			m.LimitTheta((low-x+m.HarrisEpsForBound(upper))/d, theta, unlimited)
		} else if m.BelowBound(x, upper) {
			m.LimitTheta((upper-x+m.HarrisEpsForBound(upper))/d, theta, unlimited)
		} else if !m.AboveBound(x, upper) {
			*theta = 0
			*unlimited = false
		}
	} else {
		x, upper, err := m.columnBound(j, true)
		if err != nil {
			return err
		}
		if m.AboveBound(x, upper) {
			m.LimitTheta((upper-x-m.HarrisEpsForBound(upper))/d, theta, unlimited)
		} else {
			_, low, err := m.columnBound(j, false)
			if err != nil {
				return err
			}
			if m.AboveBound(x, low) {
				m.LimitTheta((low-x-m.HarrisEpsForBound(low))/d, theta, unlimited)
			} else if !m.BelowBound(x, low) {
				*theta = 0
				*unlimited = false
			}
		}
	}
	return nil
}

// LimitThetaOnBasisColumn uses the cached global feasibility state, as the
// source does. Output pointers are required; malformed source indices return
// ErrBounds after any preceding source mutations.
func (m *FloatPrimalCoreSolver) LimitThetaOnBasisColumn(j uint32, d float64, theta *float64, unlimited *bool) error {
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return ErrBounds
	}
	var err error
	feasible := m.CurrentXIsFeasible()
	switch (*m.ColumnTypes)[j] {
	case ColumnFree:
	case ColumnUpperBound:
		if !feasible || d > 0 {
			err = m.harrisOneBound(j, d, true, feasible, theta, unlimited)
		}
	case ColumnLowBound:
		if !feasible || d < 0 {
			err = m.harrisOneBound(j, d, false, feasible, theta, unlimited)
		}
	case ColumnFixed, ColumnBoxed:
		if feasible {
			err = m.harrisOneBound(j, d, d > 0, true, theta, unlimited)
		} else {
			err = m.harrisBoxed(j, d, theta, unlimited)
		}
	default:
		err = sourceAssertionFailure()
	}
	if err != nil {
		return err
	}
	if !*unlimited && *theta < 0 {
		*theta = 0
	}
	return nil
}

func (m *FloatPrimalCoreSolver) GetHarrisTheta(theta *float64) (bool, error) {
	unlimited := true
	for _, i := range m.Ed.Index {
		if uint64(i) >= uint64(len(m.Ed.Data)) {
			return unlimited, ErrBounds
		}
		ed := m.Ed.Data[i]
		if m.Settings.FloatPivotSmall(ed) {
			continue
		}
		if uint64(i) >= uint64(len(*m.Basis)) {
			return unlimited, ErrBounds
		}
		if err := m.LimitThetaOnBasisColumn((*m.Basis)[i], -ed*float64(m.SignOfEnteringDelta), theta, &unlimited); err != nil {
			return unlimited, err
		}
		if !unlimited && *theta == 0 {
			break
		}
	}
	return unlimited, nil
}

func (m *FloatPrimalCoreSolver) FindLeavingOnHarrisTheta(harrisTheta float64, t *float64) (int32, error) {
	leaving := int32(-1)
	pivotMax := 0.0
	m.ZeroHarrisEps()
	random := m.Settings.RandomNext()
	steps := len(m.Ed.Index)
	if steps == 0 {
		return leaving, ErrBounds
	}
	start := int(random % uint32(steps))
	for offset := 0; offset < steps; offset++ {
		i := m.Ed.Index[(start+offset)%steps]
		if uint64(i) >= uint64(len(m.Ed.Data)) {
			return leaving, ErrBounds
		}
		ed := m.Ed.Data[i]
		if m.Settings.FloatPivotSmall(ed) {
			continue
		}
		if uint64(i) >= uint64(len(*m.Basis)) {
			return leaving, ErrBounds
		}
		j := (*m.Basis)[i]
		ratio := 0.0
		unlimited := true
		if err := m.LimitThetaOnBasisColumn(j, -ed*float64(m.SignOfEnteringDelta), &ratio, &unlimited); err != nil {
			return leaving, err
		}
		if !unlimited && ratio <= harrisTheta && (leaving == -1 || math.Abs(ed) > pivotMax) {
			*t = ratio
			leaving = int32(j)
			pivotMax = math.Abs(ed)
		}
	}
	m.RestoreHarrisEps()
	return leaving, nil
}

// TryJumpToAnotherBoundOnEntering publishes t even when the flip is rejected.
// The source unlimited reference is read-only in this routine.
func (m *FloatPrimalCoreSolver) TryJumpToAnotherBoundOnEntering(j uint32, theta float64, t *float64, unlimited bool) (bool, error) {
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	typ := (*m.ColumnTypes)[j]
	positive := m.SignOfEnteringDelta > 0
	if typ != ColumnBoxed && !(typ == ColumnUpperBound && positive) && !(typ == ColumnLowBound && m.SignOfEnteringDelta < 0) {
		return false, nil
	}
	x, b, err := m.columnBound(j, positive)
	if err != nil {
		return false, err
	}
	if positive {
		*t = b - x
	} else {
		*t = x - b
	}
	if unlimited || *t <= theta {
		if sourceDebugEnabled && !(*t >= 0) {
			return false, ErrAssertion
		}
		return true, nil
	}
	return false, nil
}
