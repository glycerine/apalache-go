package lp

// Source: src/util/lp/lp_dual_core_solver.hpp:264-295,599-660;
// lp_core_solver_base.hpp:894-917. Concrete retained mpq/mpq.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalDualCoreSolver) compareDualValues(a, b *numeral.Rational) (cmp int, err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return 0, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return m.manager.Compare(a, b)
}
func (m *RationalDualCoreSolver) breakpointProduct(j uint32, upper bool) (yes bool, err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 3)
	scalar, tmp, product := &m.priceWork.Data[0], &m.priceWork.Data[1], &m.priceWork.Data[2]
	if err = m.manager.SetInt64(scalar, int64(m.SignOfAlphaR)); err != nil {
		return false, err
	}
	if err = etaRationalProduct(m.manager, tmp, product, scalar, &m.PivotRow.Data[j]); err != nil {
		return false, err
	}
	zero := numeral.NewRational()
	var cmp int
	if upper {
		cmp, e = m.manager.Compare(product, &zero)
	} else {
		cmp, e = m.manager.Compare(&zero, product)
	}
	if e != nil {
		return false, e
	}
	if err = m.priceWork.Close(); err != nil {
		return false, err
	}
	m.priceWork = nil
	return cmp < 0, nil
}
func (m *RationalDualCoreSolver) breakpointBoundDifferenceIsZero(j uint32, upper bool) (zero bool, err error) {
	bounds := m.LowBounds
	if upper {
		bounds = m.UpperBounds
	}
	if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*bounds)) {
		return false, ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 2)
	if err = etaRationalDifference(m.manager, &m.priceWork.Data[0], &m.priceWork.Data[1], &(*m.X)[j], &(*bounds)[j]); err != nil {
		return false, err
	}
	zero, err = m.manager.IsZero(&m.priceWork.Data[1])
	if err != nil {
		return false, err
	}
	if err = m.priceWork.Close(); err != nil {
		return false, err
	}
	m.priceWork = nil
	return zero, nil
}
func (m *RationalDualCoreSolver) CanBeBreakpoint(j uint32) (yes bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	if uint64(j) >= uint64(len(m.PivotRow.Data)) {
		return false, ErrBounds
	}
	zero, e := m.manager.IsZero(&m.PivotRow.Data[j])
	if e != nil {
		return false, e
	}
	if zero {
		return false, nil
	}
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	switch (*m.ColumnTypes)[j] {
	case ColumnLowBound, ColumnUpperBound:
		upper := (*m.ColumnTypes)[j] == ColumnUpperBound
		if sourceDebugEnabled {
			ok, e := m.breakpointBoundDifferenceIsZero(j, upper)
			if e != nil {
				return false, e
			}
			if !ok {
				return false, ErrAssertion
			}
		}
		return m.breakpointProduct(j, upper)
	case ColumnBoxed:
		low, e := m.dualAtBound(j, false)
		if e != nil {
			return false, e
		}
		growing, e := m.breakpointProduct(j, false)
		return low == growing, e
	case ColumnFree:
		return true, nil
	default:
		return false, nil
	}
}
func (m *RationalDualCoreSolver) FillBreakpointSet() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	m.BreakpointSet = make(map[uint32]bool)
	for _, j := range *m.NBasis {
		yes, e := m.CanBeBreakpoint(j)
		if e != nil {
			return e
		}
		if yes {
			m.BreakpointSet[j] = true
		}
	}
	return nil
}
func (m *RationalDualCoreSolver) dualAtBound(j uint32, upper bool) (bool, error) {
	bounds := m.LowBounds
	if upper {
		bounds = m.UpperBounds
	}
	if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*bounds)) {
		return false, ErrBounds
	}
	return m.AtBound(&(*m.X)[j], &(*bounds)[j])
}

// GetNonBasicColumnValuePosition retains source short-circuit bound reads.
func (m *RationalCoreSolverBase) GetNonBasicColumnValuePosition(j uint32) (position NonBasicColumnValuePosition, err error) {
	if err = m.ready(); err != nil {
		return PositionAtLowBound, err
	}
	defer func() { m.stopped = err != nil }()
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return PositionAtLowBound, ErrBounds
	}
	typ := (*m.ColumnTypes)[j]
	if typ == ColumnFree {
		return PositionFreeOfBounds, nil
	}
	if typ != ColumnFixed && typ != ColumnBoxed && typ != ColumnLowBound && typ != ColumnUpperBound {
		return PositionAtLowBound, sourceAssertionFailure()
	}
	if uint64(j) >= uint64(len(*m.X)) {
		return PositionAtLowBound, ErrBounds
	}
	bounds := m.LowBounds
	if typ == ColumnUpperBound {
		bounds = m.UpperBounds
	}
	if uint64(j) >= uint64(len(*bounds)) {
		return PositionAtLowBound, ErrBounds
	}
	at, e := m.AtBound(&(*m.X)[j], &(*bounds)[j])
	if e != nil {
		return PositionAtLowBound, e
	}
	if at {
		switch typ {
		case ColumnFixed:
			return PositionAtFixed, nil
		case ColumnUpperBound:
			return PositionAtUpperBound, nil
		default:
			return PositionAtLowBound, nil
		}
	}
	if typ == ColumnBoxed {
		if uint64(j) >= uint64(len(*m.UpperBounds)) {
			return PositionAtLowBound, ErrBounds
		}
		at, e = m.AtBound(&(*m.X)[j], &(*m.UpperBounds)[j])
		if e != nil {
			return PositionAtLowBound, e
		}
		if at {
			return PositionAtUpperBound, nil
		}
	}
	return PositionNotAtBound, nil
}

// dualBreakpointTime owns only its scalar expressions. Endpoint checks finish
// with the base owner before copied-left max/min, shift and quotient begin.
func (m *RationalDualCoreSolver) dualBreakpointTime(j uint32, harris bool, out *numeral.Rational) (err error) {
	low, e := m.dualAtBound(j, false)
	if e != nil {
		return e
	}
	if uint64(j) >= uint64(len(m.D.Data)) {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 5)
	zero, tmp, shift, quotient, absolute := &m.priceWork.Data[0], &m.priceWork.Data[1], &m.priceWork.Data[2], &m.priceWork.Data[3], &m.priceWork.Data[4]
	d := &m.D.Data[j]
	var cmp int
	if low {
		cmp, e = m.manager.Compare(d, zero)
	} else {
		cmp, e = m.manager.Compare(zero, d)
	}
	if e != nil {
		return e
	}
	chosen := d
	if cmp < 0 {
		chosen = zero
	}
	if harris {
		if low {
			err = etaRationalSum(m.manager, tmp, shift, chosen, &m.HarrisTolerance)
		} else {
			err = etaRationalDifference(m.manager, tmp, shift, chosen, &m.HarrisTolerance)
		}
		if err != nil {
			return err
		}
		chosen = shift
	}
	if uint64(j) >= uint64(len(m.PivotRow.Data)) {
		return ErrBounds
	}
	if err = etaRationalQuotient(m.manager, tmp, quotient, chosen, &m.PivotRow.Data[j]); err != nil {
		return err
	}
	if err = m.manager.Copy(absolute, quotient); err != nil {
		return err
	}
	if err = m.manager.Abs(absolute); err != nil {
		return err
	}
	if err = m.manager.Copy(out, absolute); err != nil {
		return err
	}
	if err = m.priceWork.Close(); err != nil {
		return err
	}
	m.priceWork = nil
	return nil
}
func (m *RationalDualCoreSolver) CalculateHarrisDeltaOnBreakpointSet(out *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.harrisValues != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	// ret is constructed before the source nonempty assertion.
	m.harrisValues = NewIndexedRationalVector(m.manager, 2)
	ret, t := &m.harrisValues.Data[0], &m.harrisValues.Data[1]
	if sourceDebugEnabled && len(m.BreakpointSet) == 0 {
		return ErrAssertion
	}
	first := true
	for _, j := range dualOrderedKeys(m.BreakpointSet) {
		if err = m.dualBreakpointTime(j, true, t); err != nil {
			return err
		}
		take := first
		if !first {
			cmp, e := m.compareDualValues(t, ret)
			if e != nil {
				return e
			}
			take = cmp < 0
		}
		if take {
			if err = m.copyPricingValue(ret, t); err != nil {
				return err
			}
			first = false
		}
		// Each source loop iteration destroys its local t.
		if err = m.manager.Del(t); err != nil {
			return err
		}
		*t = numeral.NewRational()
	}
	if err = m.copyPricingValue(out, ret); err != nil {
		return err
	}
	if err = m.harrisValues.Close(); err != nil {
		return err
	}
	m.harrisValues = nil
	return nil
}
func (m *RationalDualCoreSolver) FillTightSetOnHarrisDelta(delta *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.tightValue != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	m.TightSet = make(map[uint32]bool)
	for _, j := range dualOrderedKeys(m.BreakpointSet) {
		m.tightValue = NewIndexedRationalVector(m.manager, 1)
		if err = m.dualBreakpointTime(j, false, &m.tightValue.Data[0]); err != nil {
			return err
		}
		cmp, e := m.compareDualValues(delta, &m.tightValue.Data[0])
		if e != nil {
			return e
		}
		if cmp >= 0 {
			m.TightSet[j] = true
		}
		if err = m.tightValue.Close(); err != nil {
			return err
		}
		m.tightValue = nil
	}
	return nil
}
func (m *RationalDualCoreSolver) FindQOnTightSet() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.qValues != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	m.Q = -1
	m.qValues = NewIndexedRationalVector(m.manager, 2)
	max, r := &m.qValues.Data[0], &m.qValues.Data[1]
	for _, j := range dualOrderedKeys(m.TightSet) {
		if uint64(j) >= uint64(len(m.PivotRow.Data)) {
			return ErrBounds
		}
		if err = m.dualAbsValue(r, &m.PivotRow.Data[j]); err != nil {
			return err
		}
		take := m.Q == -1
		if !take {
			cmp, e := m.compareDualValues(max, r)
			if e != nil {
				return e
			}
			take = cmp < 0
		}
		if take {
			if err = m.copyPricingValue(max, r); err != nil {
				return err
			}
			m.Q = int32(j)
		}
		if err = m.manager.Del(r); err != nil {
			return err
		}
		*r = numeral.NewRational()
	}
	delete(m.TightSet, uint32(m.Q))
	if sourceDebugEnabled && m.Q == -1 {
		return ErrAssertion
	}
	if err = m.qValues.Close(); err != nil {
		return err
	}
	m.qValues = nil
	return nil
}
func (m *RationalDualCoreSolver) FindQAndTightSet() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.harrisDelta != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	m.harrisDelta = NewIndexedRationalVector(m.manager, 1)
	if err = m.CalculateHarrisDeltaOnBreakpointSet(&m.harrisDelta.Data[0]); err != nil {
		return err
	}
	if err = m.FillTightSetOnHarrisDelta(&m.harrisDelta.Data[0]); err != nil {
		return err
	}
	if err = m.FindQOnTightSet(); err != nil {
		return err
	}
	position, e := m.GetNonBasicColumnValuePosition(uint32(m.Q))
	if e != nil {
		return e
	}
	m.EnteringBoundaryPosition = position
	if err = m.harrisDelta.Close(); err != nil {
		return err
	}
	m.harrisDelta = nil
	return nil
}

// dualAbsValue preserves the copied input and the separate returned value.
func (m *RationalDualCoreSolver) dualAbsValue(out, in *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 1)
	tmp := &m.priceWork.Data[0]
	if err = m.manager.Copy(tmp, in); err != nil {
		return err
	}
	if err = m.manager.Abs(tmp); err != nil {
		return err
	}
	if err = m.manager.Copy(out, tmp); err != nil {
		return err
	}
	if err = m.priceWork.Close(); err != nil {
		return err
	}
	m.priceWork = nil
	return nil
}
