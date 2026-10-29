package lp

// Source: src/util/lp/lp_dual_core_solver.hpp:237-337,558-708;
// lp_core_solver_base.hpp:894-917. Retained floating dual ratio test.

import (
	"math"
	"sort"
)

type NonBasicColumnValuePosition int32

const (
	PositionAtLowBound NonBasicColumnValuePosition = iota
	PositionAtUpperBound
	PositionAtFixed
	PositionFreeOfBounds
	PositionNotAtBound
)

// Source std::set traversal is ascending, including equal-pivot tie breaks.
func dualOrderedKeys(set map[uint32]bool) []uint32 {
	keys := make([]uint32, 0, len(set))
	for j := range set {
		keys = append(keys, j)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func (m *FloatCoreSolverBase) dualBound(j uint32, upper bool) (float64, float64, error) {
	bounds := m.LowBounds
	if upper {
		bounds = m.UpperBounds
	}
	if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*bounds)) {
		return 0, 0, ErrBounds
	}
	return (*m.X)[j], (*bounds)[j], nil
}
func (m *FloatCoreSolverBase) dualAtBound(j uint32, upper bool) (bool, error) {
	x, b, err := m.dualBound(j, upper)
	if err != nil {
		return false, err
	}
	return m.AtBound(x, b), nil
}
func (m *FloatCoreSolverBase) BoundSpan(j uint32) (float64, error) {
	if uint64(j) >= uint64(len(*m.LowBounds)) || uint64(j) >= uint64(len(*m.UpperBounds)) {
		return 0, ErrBounds
	}
	return (*m.UpperBounds)[j] - (*m.LowBounds)[j], nil
}
func (m *FloatCoreSolverBase) GetNonBasicColumnValuePosition(j uint32) (NonBasicColumnValuePosition, error) {
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return PositionAtLowBound, ErrBounds
	}
	typ := (*m.ColumnTypes)[j]
	if typ == ColumnFree {
		return PositionFreeOfBounds, nil
	}
	if typ == ColumnUpperBound {
		ok, err := m.dualAtBound(j, true)
		if err != nil {
			return PositionAtLowBound, err
		}
		if ok {
			return PositionAtUpperBound, nil
		}
		return PositionNotAtBound, nil
	}
	if typ != ColumnFixed && typ != ColumnLowBound && typ != ColumnBoxed {
		return PositionAtLowBound, sourceAssertionFailure()
	}
	ok, err := m.dualAtBound(j, false)
	if err != nil {
		return PositionAtLowBound, err
	}
	if ok {
		if typ == ColumnFixed {
			return PositionAtFixed, nil
		}
		return PositionAtLowBound, nil
	}
	if typ == ColumnBoxed {
		ok, err = m.dualAtBound(j, true)
		if err != nil {
			return PositionAtLowBound, err
		}
		if ok {
			return PositionAtUpperBound, nil
		}
	}
	return PositionNotAtBound, nil
}

func (m *FloatDualCoreSolver) DefineSignOfAlphaR() (int, error) {
	p := uint32(m.P)
	if uint64(p) >= uint64(len(*m.ColumnTypes)) {
		return 0, ErrBounds
	}
	switch (*m.ColumnTypes)[p] {
	case ColumnFixed, ColumnBoxed:
		x, b, err := m.dualBound(p, false)
		if err != nil {
			return 0, err
		}
		if m.BelowBound(x, b) {
			return -1, nil
		}
		x, b, err = m.dualBound(p, true)
		if err != nil {
			return 0, err
		}
		if m.AboveBound(x, b) {
			return 1, nil
		}
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
		fallthrough
	case ColumnLowBound:
		x, b, err := m.dualBound(p, false)
		if err != nil {
			return 0, err
		}
		if m.BelowBound(x, b) {
			return -1, nil
		}
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
		fallthrough
	case ColumnUpperBound:
		x, b, err := m.dualBound(p, true)
		if err != nil {
			return 0, err
		}
		if m.AboveBound(x, b) {
			return 1, nil
		}
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
		fallthrough
	default:
		return 0, sourceAssertionFailure()
	}
}
func (m *FloatDualCoreSolver) GetDelta() (float64, error) {
	p := uint32(m.P)
	if uint64(p) >= uint64(len(*m.ColumnTypes)) {
		return 0, ErrBounds
	}
	switch (*m.ColumnTypes)[p] {
	case ColumnBoxed:
		x, b, err := m.dualBound(p, false)
		if err != nil {
			return 0, err
		}
		if m.BelowBound(x, b) {
			return x - b, nil
		}
		x, b, err = m.dualBound(p, true)
		if err != nil {
			return 0, err
		}
		if m.AboveBound(x, b) {
			return x - b, nil
		}
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
		fallthrough
	case ColumnLowBound:
		x, b, err := m.dualBound(p, false)
		if err != nil {
			return 0, err
		}
		if m.BelowBound(x, b) {
			return x - b, nil
		}
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
		fallthrough
	case ColumnUpperBound:
		x, b, err := m.dualBound(p, true)
		if err != nil {
			return 0, err
		}
		if m.AboveBound(x, b) {
			return m.GetEdgeSteepnessForUpperBound(p)
		}
		if err := sourceAssertionFailure(); err != nil {
			return 0, err
		}
		fallthrough
	case ColumnFixed:
		x, b, err := m.dualBound(p, true)
		return x - b, err
	default:
		return 0, sourceAssertionFailure()
	}
}

func (m *FloatDualCoreSolver) CanBeBreakpoint(j uint32) (bool, error) {
	if uint64(j) >= uint64(len(m.PivotRow.Data)) {
		return false, ErrBounds
	}
	a := m.PivotRow.Data[j]
	if m.Settings.FloatPivotSmall(a) {
		return false, nil
	}
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	switch (*m.ColumnTypes)[j] {
	case ColumnLowBound, ColumnUpperBound:
		upper := (*m.ColumnTypes)[j] == ColumnUpperBound
		if sourceDebugEnabled {
			x, b, err := m.dualBound(j, upper)
			if err != nil {
				return false, err
			}
			if !m.Settings.FloatHarrisSmall(x - b) {
				return false, ErrAssertion
			}
		}
		if upper {
			return float64(m.SignOfAlphaR)*a < 0, nil
		}
		return float64(m.SignOfAlphaR)*a > 0, nil
	case ColumnBoxed:
		low, err := m.dualAtBound(j, false)
		return low == (float64(m.SignOfAlphaR)*a > 0), err
	case ColumnFree:
		return true, nil
	default:
		return false, nil
	}
}
func (m *FloatDualCoreSolver) FillBreakpointSet() error {
	m.BreakpointSet = make(map[uint32]bool)
	for _, j := range *m.NBasis {
		ok, err := m.CanBeBreakpoint(j)
		if err != nil {
			return err
		}
		if ok {
			m.BreakpointSet[j] = true
		}
	}
	return nil
}
func (m *FloatDualCoreSolver) DeltaKeepsTheSign(sign int, delta float64) bool {
	return delta > m.Settings.ZeroTolerance && sign == 1 || delta < -m.Settings.ZeroTolerance && sign == -1
}
func (m *FloatDualCoreSolver) SetStatusToTentativeDualUnboundedOrDualUnbounded() {
	if m.Status == StatusTentativeDualUnbounded {
		m.Status = StatusDualUnbounded
	} else {
		m.Status = StatusTentativeDualUnbounded
	}
}

// Keep the first operand on NaN/ties, as std::max(d,0)/std::min(d,0).
func (m *FloatDualCoreSolver) dualBreakpointTime(j uint32, harris bool) (float64, error) {
	low, err := m.dualAtBound(j, false)
	if err != nil {
		return 0, err
	}
	if uint64(j) >= uint64(len(m.D)) || uint64(j) >= uint64(len(m.PivotRow.Data)) {
		return 0, ErrBounds
	}
	d := m.D[j]
	if low {
		if d < 0 {
			d = 0
		}
		if harris {
			d += m.HarrisTolerance
		}
	} else {
		if d > 0 {
			d = 0
		}
		if harris {
			d -= m.HarrisTolerance
		}
	}
	return math.Abs(d / m.PivotRow.Data[j]), nil
}
func (m *FloatDualCoreSolver) CalculateHarrisDeltaOnBreakpointSet() (float64, error) {
	if sourceDebugEnabled && len(m.BreakpointSet) == 0 {
		return 0, ErrAssertion
	}
	ret := 0.0
	first := true
	for _, j := range dualOrderedKeys(m.BreakpointSet) {
		v, err := m.dualBreakpointTime(j, true)
		if err != nil {
			return 0, err
		}
		if first {
			ret = v
			first = false
		} else if v < ret {
			ret = v
		}
	}
	return ret, nil
}
func (m *FloatDualCoreSolver) FillTightSetOnHarrisDelta(delta float64) error {
	m.TightSet = make(map[uint32]bool)
	for _, j := range dualOrderedKeys(m.BreakpointSet) {
		v, err := m.dualBreakpointTime(j, false)
		if err != nil {
			return err
		}
		if v <= delta {
			m.TightSet[j] = true
		}
	}
	return nil
}
func (m *FloatDualCoreSolver) FindQOnTightSet() error {
	m.Q = -1
	max := 0.0
	for _, j := range dualOrderedKeys(m.TightSet) {
		if uint64(j) >= uint64(len(m.PivotRow.Data)) {
			return ErrBounds
		}
		r := math.Abs(m.PivotRow.Data[j])
		if m.Q == -1 || r > max {
			max = r
			m.Q = int32(j)
		}
	}
	delete(m.TightSet, uint32(m.Q))
	if sourceDebugEnabled && m.Q == -1 {
		return ErrAssertion
	}
	return nil
}
func (m *FloatDualCoreSolver) FindQAndTightSet() error {
	delta, err := m.CalculateHarrisDeltaOnBreakpointSet()
	if err != nil {
		return err
	}
	if err = m.FillTightSetOnHarrisDelta(delta); err != nil {
		return err
	}
	if err = m.FindQOnTightSet(); err != nil {
		return err
	}
	position, err := m.GetNonBasicColumnValuePosition(uint32(m.Q))
	if err != nil {
		return err
	}
	m.EnteringBoundaryPosition = position
	return nil
}
func (m *FloatDualCoreSolver) TightBreakpointsAreAllBoxed() (bool, error) {
	keys := append([]uint32{uint32(m.Q)}, dualOrderedKeys(m.TightSet)...)
	for _, j := range keys {
		if uint64(j) >= uint64(len(*m.ColumnTypes)) {
			return false, ErrBounds
		}
		if (*m.ColumnTypes)[j] != ColumnBoxed {
			return false, nil
		}
	}
	return true, nil
}
func (m *FloatDualCoreSolver) DeltaLostOnFlipsOfTightBreakpoints() (float64, error) {
	ret := 0.0
	keys := append([]uint32{uint32(m.Q)}, dualOrderedKeys(m.TightSet)...)
	for i, j := range keys {
		span, err := m.BoundSpan(j)
		if err != nil {
			return 0, err
		}
		if uint64(j) >= uint64(len(m.PivotRow.Data)) {
			return 0, ErrBounds
		}
		v := math.Abs(span * m.PivotRow.Data[j])
		if i == 0 {
			ret = v
		} else {
			ret += v
		}
	}
	return ret, nil
}
func (m *FloatDualCoreSolver) AddTightBreakpointsAndQToFlippedSet() {
	if m.FlippedBoxed == nil {
		m.FlippedBoxed = make(map[uint32]bool)
	}
	m.FlippedBoxed[uint32(m.Q)] = true
	for j := range m.TightSet {
		m.FlippedBoxed[j] = true
	}
}
func (m *FloatDualCoreSolver) EraseTightBreakpointsAndQFromBreakpointSet() {
	delete(m.BreakpointSet, uint32(m.Q))
	for j := range m.TightSet {
		delete(m.BreakpointSet, j)
	}
}
func (m *FloatDualCoreSolver) RatioTest() (bool, error) {
	sign, err := m.DefineSignOfAlphaR()
	if err != nil {
		return false, err
	}
	m.SignOfAlphaR = sign
	if err = m.FillBreakpointSet(); err != nil {
		return false, err
	}
	m.FlippedBoxed = make(map[uint32]bool)
	initial := 1
	if !(m.Delta >= 0) {
		initial = -1
	}
	for {
		if len(m.BreakpointSet) == 0 {
			m.SetStatusToTentativeDualUnboundedOrDualUnbounded()
			return false, nil
		}
		m.Status = StatusFeasible
		if err = m.FindQAndTightSet(); err != nil {
			return false, err
		}
		boxed, err := m.TightBreakpointsAreAllBoxed()
		if err != nil {
			return false, err
		}
		if !boxed {
			break
		}
		lost, err := m.DeltaLostOnFlipsOfTightBreakpoints()
		if err != nil {
			return false, err
		}
		del := m.Delta - lost*float64(initial)
		if !m.DeltaKeepsTheSign(initial, del) {
			break
		}
		if len(m.TightSet)+1 == len(m.BreakpointSet) {
			break
		}
		m.AddTightBreakpointsAndQToFlippedSet()
		m.Delta = del
		m.EraseTightBreakpointsAndQFromBreakpointSet()
	}
	q := uint32(m.Q)
	if uint64(q) >= uint64(len(m.D)) || uint64(q) >= uint64(len(m.PivotRow.Data)) {
		return false, ErrBounds
	}
	m.ThetaD = m.D[q] / m.PivotRow.Data[q]
	return true, nil
}
