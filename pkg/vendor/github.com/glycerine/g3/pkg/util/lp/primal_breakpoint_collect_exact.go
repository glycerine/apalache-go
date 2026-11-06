package lp

// Source: src/util/lp/lp_primal_core_solver.h:765-769;
// lp_primal_core_solver.hpp:1074-1085,1252-1272,1337-1364.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *PairPrimalCoreSolver) sameSignRational(a *numeral.Rational) (bool, error) {
	sign, err := m.manager.Sign(a)
	return (sign > 0 && m.SignOfEnteringDelta > 0) || (sign < 0 && m.SignOfEnteringDelta < 0), err
}
func (m *PairPrimalCoreSolver) sameSignPair(a *RationalPair) (bool, error) {
	sign, err := a.Sign(m.manager)
	return (sign > 0 && m.SignOfEnteringDelta > 0) || (sign < 0 && m.SignOfEnteringDelta < 0), err
}

// buildBreakpointDelta retains both source locals, diff and delta_j. Scalar
// expression temporaries end before AddBreakpoint starts its own captures.
func (m *PairPrimalCoreSolver) buildBreakpointDelta(x, bound *RationalPair, d *numeral.Rational) (zero bool, err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 6)
	m.pairWork = NewIndexedPairVector(m.manager, 2)
	tmp, dx, dy, scalar, qx, qy := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2], &m.work.Data[3], &m.work.Data[4], &m.work.Data[5]
	if err = etaRationalDifference(m.manager, tmp, dx, &x.X, &bound.X); err != nil {
		return false, err
	}
	if err = etaRationalDifference(m.manager, tmp, dy, &x.Y, &bound.Y); err != nil {
		return false, err
	}
	diff, delta := &m.pairWork.Data[0], &m.pairWork.Data[1]
	if err = m.manager.Copy(&diff.X, dx); err != nil {
		return false, err
	}
	if err = m.manager.Copy(&diff.Y, dy); err != nil {
		return false, err
	}
	if err = m.manager.Del(dx); err != nil {
		return false, err
	}
	if err = m.manager.Del(dy); err != nil {
		return false, err
	}
	zero, err = diff.IsZero(m.manager)
	if err != nil {
		return false, err
	}
	if !zero {
		if err = m.manager.Copy(scalar, d); err != nil {
			return false, err
		}
		if err = etaRationalQuotient(m.manager, tmp, qx, &diff.X, scalar); err != nil {
			return false, err
		}
		if err = etaRationalQuotient(m.manager, tmp, qy, &diff.Y, scalar); err != nil {
			return false, err
		}
		if err = m.manager.Copy(&delta.X, qx); err != nil {
			return false, err
		}
		if err = m.manager.Copy(&delta.Y, qy); err != nil {
			return false, err
		}
	}
	if err = m.work.Close(); err != nil {
		return false, err
	}
	m.work = nil
	return zero, nil
}

func (m *PairPrimalCoreSolver) TryAddBreakpoint(j uint32, x *RationalPair, d *numeral.Rational, typ BreakpointType, bound *RationalPair) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.tryAddBreakpointValue(j, x, d, typ, bound)
}

func (m *PairPrimalCoreSolver) tryAddBreakpointValue(j uint32, x *RationalPair, d *numeral.Rational, typ BreakpointType, bound *RationalPair) (err error) {
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
		add, err = m.sameSignPair(&m.pairWork.Data[1])
		if err != nil {
			return err
		}
	}
	if add {
		// delta_j is initialized zero in the equality branch.
		if err = m.addBreakpointValue(j, &m.pairWork.Data[1], typ); err != nil {
			return err
		}
	}
	if err = m.pairWork.Close(); err != nil {
		return err
	}
	m.pairWork = nil
	return nil
}

func (m *PairPrimalCoreSolver) TryAddBreakpointInRow(i uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.tryAddBreakpointInRowValue(i)
}

func (m *PairPrimalCoreSolver) tryAddBreakpointInRowValue(i uint32) error {
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

func (m *PairPrimalCoreSolver) buildEnteringBreakpointSpan(j uint32) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.work = NewIndexedRationalVector(m.manager, 3)
	m.pairWork = NewIndexedPairVector(m.manager, 2)
	tmp, dx, dy := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2]
	low, upper := &(*m.LowBounds)[j], &(*m.UpperBounds)[j]
	if err = etaRationalDifference(m.manager, tmp, dx, &upper.X, &low.X); err != nil {
		return err
	}
	if err = etaRationalDifference(m.manager, tmp, dy, &upper.Y, &low.Y); err != nil {
		return err
	}
	span, result := &m.pairWork.Data[0], &m.pairWork.Data[1]
	if err = m.manager.Copy(&span.X, dx); err != nil {
		return err
	}
	if err = m.manager.Copy(&span.Y, dy); err != nil {
		return err
	}
	if m.SignOfEnteringDelta < 0 {
		if err = m.manager.Copy(dx, &span.X); err != nil {
			return err
		}
		if err = m.manager.Neg(dx); err != nil {
			return err
		}
		if err = m.manager.Copy(dy, &span.Y); err != nil {
			return err
		}
		if err = m.manager.Neg(dy); err != nil {
			return err
		}
		if err = m.manager.Copy(&result.X, dx); err != nil {
			return err
		}
		if err = m.manager.Copy(&result.Y, dy); err != nil {
			return err
		}
	} else {
		if err = result.Copy(m.manager, span); err != nil {
			return err
		}
	}
	if err = m.work.Close(); err != nil {
		return err
	}
	m.work = nil
	return nil
}

func (m *PairPrimalCoreSolver) FillBreakpointsArray(entering uint32) (err error) {
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
	if err = m.addBreakpointValue(entering, &m.pairWork.Data[1], typ); err != nil {
		return err
	}
	if err = m.pairWork.Close(); err != nil {
		return err
	}
	m.pairWork = nil
	return nil
}
