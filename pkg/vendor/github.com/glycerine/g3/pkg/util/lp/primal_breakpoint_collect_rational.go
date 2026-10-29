package lp

// Source: src/util/lp/lp_primal_core_solver.h:765-769;
// lp_primal_core_solver.hpp:1074-1085,1252-1272,1337-1364.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalPrimalCoreSolver) sameSignRational(a *numeral.Rational) (bool, error) {
	sign, err := m.manager.Sign(a)
	return (sign > 0 && m.SignOfEnteringDelta > 0) || (sign < 0 && m.SignOfEnteringDelta < 0), err
}

// buildBreakpointDelta retains both source locals, diff and delta_j. Scalar
// expression temporaries end before AddBreakpoint starts its own captures.
func (m *RationalPrimalCoreSolver) buildBreakpointDelta(x, bound *numeral.Rational, d *numeral.Rational) (zero bool, err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 1)
	m.breakpointCollect = NewIndexedRationalVector(m.manager, 2)
	tmp := &m.work.Data[0]
	diff, delta := &m.breakpointCollect.Data[0], &m.breakpointCollect.Data[1]
	if err = etaRationalDifference(m.manager, tmp, diff, x, bound); err != nil {
		return false, err
	}
	zero, err = m.manager.IsZero(diff)
	if err != nil {
		return false, err
	}
	if !zero {
		// Scalar rational division borrows d; no pair-coordinate denominator copy.
		if err = etaRationalQuotient(m.manager, tmp, delta, diff, d); err != nil {
			return false, err
		}
	}
	if err = m.work.Close(); err != nil {
		return false, err
	}
	m.work = nil
	return zero, nil
}

func (m *RationalPrimalCoreSolver) TryAddBreakpoint(j uint32, x *numeral.Rational, d *numeral.Rational, typ BreakpointType, bound *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.tryAddBreakpointValue(j, x, d, typ, bound)
}

func (m *RationalPrimalCoreSolver) tryAddBreakpointValue(j uint32, x *numeral.Rational, d *numeral.Rational, typ BreakpointType, bound *numeral.Rational) (err error) {
	if x == nil || d == nil || bound == nil {
		return ErrBounds
	}
	zero, e := m.buildBreakpointDelta(x, bound, d)
	if e != nil {
		return e
	}
	add := true
	if zero {
		if typ == BreakpointLow || typ == BreakpointUpper {
			same, e := m.sameSignRational(d)
			if e != nil {
				return e
			}
			add = (typ == BreakpointLow && same) || (typ == BreakpointUpper && !same)
		}
	} else {
		add, err = m.sameSignRational(&m.breakpointCollect.Data[1])
		if err != nil {
			return err
		}
	}
	if add {
		// delta_j is initialized zero in the equality branch.
		if err = m.addBreakpointValue(j, &m.breakpointCollect.Data[1], typ); err != nil {
			return err
		}
	}
	if err = m.breakpointCollect.Close(); err != nil {
		return err
	}
	m.breakpointCollect = nil
	return nil
}

func (m *RationalPrimalCoreSolver) TryAddBreakpointInRow(i uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.tryAddBreakpointInRowValue(i)
}

func (m *RationalPrimalCoreSolver) tryAddBreakpointInRowValue(i uint32) error {
	if sourceDebugEnabled && i >= m.A.RowCount() {
		return ErrAssertion
	}
	if uint64(i) >= uint64(len(m.Ed.Data)) {
		return ErrBounds
	}
	d := &m.Ed.Data[i]
	zero, err := m.manager.IsZero(d)
	if err != nil {
		return err
	}
	if zero {
		return nil
	}
	if uint64(i) >= uint64(len(*m.Basis)) {
		return ErrBounds
	}
	j := (*m.Basis)[i]
	if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return ErrBounds
	}
	x := &(*m.X)[j]
	add := func(upper bool, typ BreakpointType) error {
		bounds := m.LowBounds
		if upper {
			bounds = m.UpperBounds
		}
		if uint64(j) >= uint64(len(*bounds)) {
			return ErrBounds
		}
		return m.tryAddBreakpointValue(j, x, d, typ, &(*bounds)[j])
	}
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed:
		return add(false, BreakpointFixed)
	case ColumnBoxed:
		if err = add(false, BreakpointLow); err != nil {
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

func (m *RationalPrimalCoreSolver) buildEnteringBreakpointSpan(j uint32) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 1)
	m.breakpointCollect = NewIndexedRationalVector(m.manager, 2)
	tmp := &m.work.Data[0]
	span, result := &m.breakpointCollect.Data[0], &m.breakpointCollect.Data[1]
	if err = etaRationalDifference(m.manager, tmp, span, &(*m.UpperBounds)[j], &(*m.LowBounds)[j]); err != nil {
		return err
	}
	if m.SignOfEnteringDelta < 0 {
		if err = m.manager.Copy(result, span); err != nil {
			return err
		}
		if err = m.manager.Neg(result); err != nil {
			return err
		}
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}

func (m *RationalPrimalCoreSolver) FillBreakpointsArray(entering uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.ClearBreakpoints(); err != nil {
		return err
	}
	for _, i := range m.Ed.Index {
		if err = m.tryAddBreakpointInRowValue(i); err != nil {
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
	if err = m.buildEnteringBreakpointSpan(entering); err != nil {
		return err
	}
	typ := BreakpointUpper
	if m.SignOfEnteringDelta < 0 {
		typ = BreakpointLow
	}
	value := &m.breakpointCollect.Data[0]
	if m.SignOfEnteringDelta < 0 {
		value = &m.breakpointCollect.Data[1]
	}
	if err = m.addBreakpointValue(entering, value, typ); err != nil {
		return err
	}
	if err = m.breakpointCollect.Close(); err != nil {
		return err
	}
	m.breakpointCollect = nil
	return nil
}
