package lp

// Source: src/util/lp/lp_dual_core_solver.hpp:109-160,236-262,301-331,
// 534-577. Concrete retained mpq/mpq dual pricing and leaving values.

import (
	"math"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// edgeSteepness preserves copied-left subtraction, compound squaring and the
// copied-left quotient. Its denominator stays borrowed through division.
func (m *RationalDualCoreSolver) edgeSteepness(p uint32, bounds *[]numeral.Rational, out *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if uint64(p) >= uint64(len(*m.Heading)) {
		return ErrBounds
	}
	r := (*m.Heading)[p]
	if sourceDebugEnabled && (r < 0 || uint32(r) >= m.A.RowCount()) {
		return ErrAssertion
	}
	if out == nil || uint64(p) >= uint64(len(*m.X)) || uint64(p) >= uint64(len(*bounds)) {
		return ErrBounds
	}
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 3)
	tmp, del, quotient := &m.priceWork.Data[0], &m.priceWork.Data[1], &m.priceWork.Data[2]
	if err = etaRationalDifference(m.manager, tmp, del, &(*m.X)[p], &(*bounds)[p]); err != nil {
		return err
	}
	if err = m.manager.Mul(del, del, del); err != nil {
		return err
	}
	if m.Betas == nil || r < 0 || uint64(r) >= uint64(len(m.Betas.Data)) {
		return ErrBounds
	}
	if err = etaRationalQuotient(m.manager, quotient, out, del, &m.Betas.Data[r]); err != nil {
		return err
	}
	if err = m.priceWork.Close(); err != nil {
		return err
	}
	m.priceWork = nil
	return nil
}
func (m *RationalDualCoreSolver) GetEdgeSteepnessForLowBound(p uint32, out *numeral.Rational) error {
	return m.edgeSteepness(p, m.LowBounds, out)
}
func (m *RationalDualCoreSolver) GetEdgeSteepnessForUpperBound(p uint32, out *numeral.Rational) error {
	return m.edgeSteepness(p, m.UpperBounds, out)
}

func (m *RationalDualCoreSolver) copyPricingValue(out, in *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return m.manager.Copy(out, in)
}
func (m *RationalDualCoreSolver) zeroPricingValue(out *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return m.manager.SetInt32(out, 0)
}
func (m *RationalDualCoreSolver) PricingForRow(i uint32, out *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	if m.pricingValue != nil {
		return ErrExactVectorPending
	}
	defer func() { m.stopped = err != nil }()
	if out == nil || uint64(i) >= uint64(len(*m.Basis)) {
		return ErrBounds
	}
	p := (*m.Basis)[i]
	if uint64(p) >= uint64(len(*m.ColumnTypes)) {
		return ErrBounds
	}
	typ := (*m.ColumnTypes)[p]
	if typ == ColumnFree {
		if sourceDebugEnabled {
			if uint64(p) >= uint64(len(m.D.Data)) {
				return ErrBounds
			}
			zero, e := m.manager.IsZero(&m.D.Data[p])
			if e != nil {
				return e
			}
			if !zero {
				return ErrAssertion
			}
		}
		return m.zeroPricingValue(out)
	}
	if typ != ColumnLowBound && typ != ColumnUpperBound && typ != ColumnBoxed && typ != ColumnFixed {
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
		return m.zeroPricingValue(out)
	}
	if uint64(p) >= uint64(len(*m.X)) {
		return ErrBounds
	}
	upper, violated := false, false
	if typ != ColumnUpperBound {
		if uint64(p) >= uint64(len(*m.LowBounds)) {
			return ErrBounds
		}
		var e error
		violated, e = m.BelowBound(&(*m.X)[p], &(*m.LowBounds)[p])
		if e != nil {
			return e
		}
	}
	if !violated && typ != ColumnLowBound {
		if uint64(p) >= uint64(len(*m.UpperBounds)) {
			return ErrBounds
		}
		var e error
		violated, e = m.AboveBound(&(*m.X)[p], &(*m.UpperBounds)[p])
		if e != nil {
			return e
		}
		upper = true
	}
	if !violated {
		return m.zeroPricingValue(out)
	}
	// The source local del is a separate return copy after the child expression.
	m.pricingValue = NewIndexedRationalVector(m.manager, 1)
	if upper {
		err = m.GetEdgeSteepnessForUpperBound(p, &m.pricingValue.Data[0])
	} else {
		err = m.GetEdgeSteepnessForLowBound(p, &m.pricingValue.Data[0])
	}
	if err != nil {
		return err
	}
	if err = m.copyPricingValue(out, &m.pricingValue.Data[0]); err != nil {
		return err
	}
	if err = m.pricingValue.Close(); err != nil {
		return err
	}
	m.pricingValue = nil
	return nil
}

// DefineSignOfAlphaR retains source fallthrough after erased assertions.
func (m *RationalDualCoreSolver) DefineSignOfAlphaR() (sign int, err error) {
	if err = m.ready(); err != nil {
		return 0, err
	}
	defer func() { m.stopped = err != nil }()
	p := uint32(m.P)
	if uint64(p) >= uint64(len(*m.ColumnTypes)) {
		return 0, ErrBounds
	}
	switch (*m.ColumnTypes)[p] {
	case ColumnBoxed, ColumnFixed:
		if uint64(p) >= uint64(len(*m.X)) || uint64(p) >= uint64(len(*m.LowBounds)) {
			return 0, ErrBounds
		}
		below, e := m.BelowBound(&(*m.X)[p], &(*m.LowBounds)[p])
		if e != nil {
			return 0, e
		}
		if below {
			return -1, nil
		}
		if uint64(p) >= uint64(len(*m.UpperBounds)) {
			return 0, ErrBounds
		}
		above, e := m.AboveBound(&(*m.X)[p], &(*m.UpperBounds)[p])
		if e != nil {
			return 0, e
		}
		if above {
			return 1, nil
		}
		if err = sourceAssertionFailure(); err != nil {
			return 0, err
		}
		fallthrough
	case ColumnLowBound:
		if uint64(p) >= uint64(len(*m.X)) || uint64(p) >= uint64(len(*m.LowBounds)) {
			return 0, ErrBounds
		}
		below, e := m.BelowBound(&(*m.X)[p], &(*m.LowBounds)[p])
		if e != nil {
			return 0, e
		}
		if below {
			return -1, nil
		}
		if err = sourceAssertionFailure(); err != nil {
			return 0, err
		}
		fallthrough
	case ColumnUpperBound:
		if uint64(p) >= uint64(len(*m.X)) || uint64(p) >= uint64(len(*m.UpperBounds)) {
			return 0, ErrBounds
		}
		above, e := m.AboveBound(&(*m.X)[p], &(*m.UpperBounds)[p])
		if e != nil {
			return 0, e
		}
		if above {
			return 1, nil
		}
		if err = sourceAssertionFailure(); err != nil {
			return 0, err
		}
		fallthrough
	default:
		if err = sourceAssertionFailure(); err != nil {
			return 0, err
		}
	}
	return 0, sourceAssertionFailure()
}
func (m *RationalDualCoreSolver) dualDifference(out, left, right *numeral.Rational) (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	m.priceWork = NewIndexedRationalVector(m.manager, 1)
	if err = etaRationalDifference(m.manager, &m.priceWork.Data[0], out, left, right); err != nil {
		return err
	}
	if err = m.priceWork.Close(); err != nil {
		return err
	}
	m.priceWork = nil
	return nil
}

// GetDelta preserves the source upper-only squared-distance result, which is
// deliberately different from the signed differences for boxed/fixed columns.
func (m *RationalDualCoreSolver) GetDelta(out *numeral.Rational) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	p := uint32(m.P)
	if out == nil || uint64(p) >= uint64(len(*m.ColumnTypes)) {
		return ErrBounds
	}
	switch (*m.ColumnTypes)[p] {
	case ColumnBoxed:
		if uint64(p) >= uint64(len(*m.X)) || uint64(p) >= uint64(len(*m.LowBounds)) {
			return ErrBounds
		}
		below, e := m.BelowBound(&(*m.X)[p], &(*m.LowBounds)[p])
		if e != nil {
			return e
		}
		if below {
			return m.dualDifference(out, &(*m.X)[p], &(*m.LowBounds)[p])
		}
		if uint64(p) >= uint64(len(*m.UpperBounds)) {
			return ErrBounds
		}
		above, e := m.AboveBound(&(*m.X)[p], &(*m.UpperBounds)[p])
		if e != nil {
			return e
		}
		if above {
			return m.dualDifference(out, &(*m.X)[p], &(*m.UpperBounds)[p])
		}
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
		fallthrough
	case ColumnLowBound:
		if uint64(p) >= uint64(len(*m.X)) || uint64(p) >= uint64(len(*m.LowBounds)) {
			return ErrBounds
		}
		below, e := m.BelowBound(&(*m.X)[p], &(*m.LowBounds)[p])
		if e != nil {
			return e
		}
		if below {
			return m.dualDifference(out, &(*m.X)[p], &(*m.LowBounds)[p])
		}
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
		fallthrough
	case ColumnUpperBound:
		if uint64(p) >= uint64(len(*m.X)) || uint64(p) >= uint64(len(*m.UpperBounds)) {
			return ErrBounds
		}
		above, e := m.AboveBound(&(*m.X)[p], &(*m.UpperBounds)[p])
		if e != nil {
			return e
		}
		if above {
			return m.GetEdgeSteepnessForUpperBound(p, out)
		}
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
		fallthrough
	case ColumnFixed:
		if uint64(p) >= uint64(len(*m.X)) || uint64(p) >= uint64(len(*m.UpperBounds)) {
			return ErrBounds
		}
		return m.dualDifference(out, &(*m.X)[p], &(*m.UpperBounds)[p])
	default:
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if err = sourceAssertionFailure(); err != nil {
		return err
	}
	return m.zeroPricingValue(out)
}
func (m *RationalDualCoreSolver) GetNumberOfRowsToTryForLeaving() (uint32, error) {
	if err := m.ready(); err != nil {
		return 0, err
	}
	s := m.A.RowCount()
	if s > 300 {
		v := (float64(s) / 100.0) * float64(m.Settings.PercentOfEnteringToCheck)
		if v < 0 || v >= float64(math.MaxUint32)+1 || math.IsNaN(v) {
			return 0, ErrBounds
		}
		s = uint32(v)
	}
	random := m.Settings.RandomNext()
	if s == 0 {
		return 0, ErrBounds
	}
	return random%s + 1, nil
}
func (m *RationalDualCoreSolver) ProblemIsDualFeasible() (ok bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	for _, j := range *m.NBasis {
		yes, e := m.ColumnIsDualFeasible(j)
		if e != nil || !yes {
			return false, e
		}
	}
	return true, nil
}
func (m *RationalDualCoreSolver) DeltaKeepsTheSign(initial int, delta *numeral.Rational) (yes bool, err error) {
	if err = m.ready(); err != nil {
		return false, err
	}
	defer func() { m.stopped = err != nil }()
	zero := numeral.NewRational()
	c, e := m.compare(delta, &zero)
	if e != nil {
		return false, e
	}
	if c > 0 && initial == 1 {
		return true, nil
	}
	c, e = m.compare(delta, &zero)
	return c < 0 && initial == -1, e
}
func (m *RationalDualCoreSolver) SetStatusToTentativeDualUnboundedOrDualUnbounded() error {
	if err := m.ready(); err != nil {
		return err
	}
	if m.Status == StatusTentativeDualUnbounded {
		m.Status = StatusDualUnbounded
	} else {
		m.Status = StatusTentativeDualUnbounded
	}
	return nil
}
