package lp

// Source: src/util/lp/breakpoint.h; lp_primal_core_solver.h:765-769;
// lp_primal_core_solver.hpp:274-303,477-487,1069-1085,1245-1272,1306-1364.

import "math"

type BreakpointType int32

const (
	BreakpointLow BreakpointType = iota
	BreakpointUpper
	BreakpointFixed
)

type FloatBreakpoint struct {
	Column uint32
	Type   BreakpointType
	Delta  float64
}

func (m *FloatPrimalCoreSolver) SameSignWithEnteringDelta(a float64) bool {
	return a > 0 && m.SignOfEnteringDelta > 0 || a < 0 && m.SignOfEnteringDelta < 0
}

func (m *FloatPrimalCoreSolver) ClearBreakpoints() error {
	m.Breakpoints = m.Breakpoints[:0]
	return m.BreakpointQueue.Clear()
}

func (m *FloatPrimalCoreSolver) AddBreakpoint(j uint32, delta float64, typ BreakpointType) error {
	m.Breakpoints = append(m.Breakpoints, FloatBreakpoint{Column: j, Type: typ, Delta: delta})
	return m.BreakpointQueue.Enqueue(m.BreakpointQueue.Size(), math.Abs(delta))
}

func (m *FloatPrimalCoreSolver) TryAddBreakpoint(j uint32, x, d float64, typ BreakpointType, bound float64) error {
	diff := x - bound
	if diff == 0 {
		switch typ {
		case BreakpointLow:
			if !m.SameSignWithEnteringDelta(d) {
				return nil
			}
		case BreakpointUpper:
			if m.SameSignWithEnteringDelta(d) {
				return nil
			}
		}
		return m.AddBreakpoint(j, 0, typ)
	}
	delta := diff / d
	if m.SameSignWithEnteringDelta(delta) {
		return m.AddBreakpoint(j, delta, typ)
	}
	return nil
}

func (m *FloatPrimalCoreSolver) TryAddBreakpointInRow(i uint32) error {
	if sourceDebugEnabled && i >= m.A.RowCount() {
		return ErrAssertion
	}
	if uint64(i) >= uint64(len(m.Ed.Data)) {
		return ErrBounds
	}
	d := m.Ed.Data[i]
	if d == 0 {
		return nil
	}
	if uint64(i) >= uint64(len(*m.Basis)) {
		return ErrBounds
	}
	j := (*m.Basis)[i]
	if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return ErrBounds
	}
	x := (*m.X)[j]
	add := func(upper bool, typ BreakpointType) error {
		_, bound, err := m.columnBound(j, upper)
		if err != nil {
			return err
		}
		return m.TryAddBreakpoint(j, x, d, typ, bound)
	}
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed:
		return add(false, BreakpointFixed)
	case ColumnBoxed:
		if err := add(false, BreakpointLow); err != nil {
			return err
		}
		return add(true, BreakpointUpper)
	case ColumnLowBound:
		return add(false, BreakpointLow)
	case ColumnUpperBound:
		return add(true, BreakpointUpper)
	case ColumnFree:
		return nil
	default:
		return sourceAssertionFailure()
	}
}

func (m *FloatPrimalCoreSolver) FillBreakpointsArray(entering uint32) error {
	if err := m.ClearBreakpoints(); err != nil {
		return err
	}
	for _, i := range m.Ed.Index {
		if err := m.TryAddBreakpointInRow(i); err != nil {
			return err
		}
	}
	if uint64(entering) >= uint64(len(*m.ColumnTypes)) {
		return ErrBounds
	}
	if (*m.ColumnTypes)[entering] != ColumnBoxed {
		return nil
	}
	if uint64(entering) >= uint64(len(*m.LowBounds)) || uint64(entering) >= uint64(len(*m.UpperBounds)) {
		return ErrBounds
	}
	span := (*m.UpperBounds)[entering] - (*m.LowBounds)[entering]
	if m.SignOfEnteringDelta < 0 {
		return m.AddBreakpoint(entering, -span, BreakpointLow)
	}
	return m.AddBreakpoint(entering, span, BreakpointUpper)
}

func (m *FloatPrimalCoreSolver) ChangeSlopeOnBreakpoint(entering uint32, b FloatBreakpoint, slope *float64) error {
	if b.Column == entering {
		if sourceDebugEnabled && (b.Type == BreakpointFixed || b.Delta == 0) {
			return ErrAssertion
		}
		*slope += float64(m.SignOfEnteringDelta)
		return nil
	}
	if uint64(b.Column) >= uint64(len(*m.Heading)) {
		return ErrBounds
	}
	i := (*m.Heading)[b.Column]
	if i < 0 {
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
		return ErrBounds
	}
	if uint64(i) >= uint64(len(m.Ed.Data)) {
		return ErrBounds
	}
	d := -m.Ed.Data[i]
	if d == 0 {
		return nil
	}
	delta := float64(m.SignOfEnteringDelta) * math.Abs(d)
	switch b.Type {
	case BreakpointFixed:
		if b.Delta == 0 {
			*slope += delta
		} else {
			*slope += 2 * delta
		}
	case BreakpointLow, BreakpointUpper:
		*slope += delta
	default:
		return sourceAssertionFailure()
	}
	return nil
}

// AdvanceOnSortedBreakpoints preserves the source double path: after changing
// the slope it always stops at the first dequeued breakpoint. The multi-step
// zero-slope randomized continuation belongs only to the exact specialization.
func (m *FloatPrimalCoreSolver) AdvanceOnSortedBreakpoints(entering uint32, t *float64) (int32, error) {
	if uint64(entering) >= uint64(len(m.D)) {
		return -1, ErrBounds
	}
	slope := m.D[entering]
	if m.BreakpointQueue.IsEmpty() {
		if err := sourceAssertionFailure(); err != nil {
			return -1, err
		}
		return -1, ErrBounds
	}
	bi, err := m.BreakpointQueue.Dequeue()
	if err != nil {
		return -1, err
	}
	if uint64(bi) >= uint64(len(m.Breakpoints)) {
		return -1, ErrBounds
	}
	b := m.Breakpoints[bi]
	if err := m.ChangeSlopeOnBreakpoint(entering, b, &slope); err != nil {
		return -1, err
	}
	*t = b.Delta
	return int32(b.Column), nil
}

func (m *FloatPrimalCoreSolver) FindLeavingAndTWithBreakpoints(entering uint32, t *float64) (int32, error) {
	if err := m.FillBreakpointsArray(entering); err != nil {
		return -1, err
	}
	return m.AdvanceOnSortedBreakpoints(entering, t)
}

func (m *FloatPrimalCoreSolver) FindLeavingAndT(entering uint32, t *float64) (int32, error) {
	if m.Settings.UseBreakpointsInFeasibilitySearch && !m.CurrentXIsFeasible() {
		return m.FindLeavingAndTWithBreakpoints(entering, t)
	}
	theta := 0.0
	unlimited, err := m.GetHarrisTheta(&theta)
	if err != nil {
		return -1, err
	}
	if sourceDebugEnabled && !unlimited && !(theta >= 0) {
		return -1, ErrAssertion
	}
	flip, err := m.TryJumpToAnotherBoundOnEntering(entering, theta, t, unlimited)
	if err != nil {
		return -1, err
	}
	if flip {
		return int32(entering), nil
	}
	if unlimited {
		return -1, nil
	}
	return m.FindLeavingOnHarrisTheta(theta, t)
}
