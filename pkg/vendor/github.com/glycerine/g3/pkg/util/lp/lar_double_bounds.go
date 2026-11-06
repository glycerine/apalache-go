package lp

// Source: src/util/lp/lar_core_solver.h:667-685,737-788.

import "github.com/glycerine/g3/pkg/util/numeral"

func (c *LARCoreSolver) LowBoundIsSet(j uint32) (bool, error) {
	return c.larBoundIsSet(j, false)
}
func (c *LARCoreSolver) UpperBoundIsSet(j uint32) (bool, error) {
	return c.larBoundIsSet(j, true)
}
func (c *LARCoreSolver) larBoundIsSet(j uint32, upper bool) (bool, error) {
	if uint64(j) >= uint64(len(c.Bounds.Types.values)) {
		return false, ErrBounds
	}
	switch c.Bounds.Types.values[j] {
	case ColumnFree:
		return false, nil
	case ColumnLowBound:
		return !upper, nil
	case ColumnUpperBound:
		return upper, nil
	case ColumnFixed, ColumnBoxed:
		return true, nil
	default:
		return false, sourceAssertionFailure()
	}
}

func (c *LARCoreSolver) UpdateDelta(delta *numeral.Rational, l, u *RationalPair) (err error) {
	if c.stopped || c.deltaWork != nil || c.calls.pending() {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	return c.updateDeltaValue(delta, l, u)
}
func (c *LARCoreSolver) updateDeltaValue(delta *numeral.Rational, l, u *RationalPair) (err error) {
	if delta == nil || l == nil || u == nil {
		return ErrBounds
	}
	before, e := c.calls.begin(c.manager)
	if e != nil {
		return e
	}
	defer func() { err = c.calls.after(c.manager, before, err) }()
	if sourceDebugEnabled {
		cmp, e := l.Compare(c.manager, u)
		if e != nil {
			return e
		}
		if cmp > 0 {
			return ErrAssertion
		}
	}
	xcmp, e := c.manager.Compare(&l.X, &u.X)
	if e != nil {
		return e
	}
	if xcmp < 0 {
		ycmp, e := c.manager.Compare(&l.Y, &u.Y)
		if e != nil {
			return e
		}
		if ycmp > 0 {
			c.deltaWork = NewIndexedRationalVector(c.manager, 3)
			w := c.deltaWork.Data
			if err = c.manager.Sub(&u.X, &l.X, &w[0]); err != nil {
				return err
			}
			if err = c.manager.Sub(&l.Y, &u.Y, &w[1]); err != nil {
				return err
			}
			if err = c.manager.Div(&w[0], &w[1], &w[2]); err != nil {
				return err
			}
			cmp, e := c.manager.Compare(&w[2], delta)
			if e != nil {
				return e
			}
			if cmp < 0 {
				if err = c.manager.Copy(delta, &w[2]); err != nil {
					return err
				}
			}
			if err = c.deltaWork.Close(); err != nil {
				return err
			}
			c.deltaWork = nil
		}
	}
	if sourceDebugEnabled {
		c.deltaWork = NewIndexedRationalVector(c.manager, 4)
		w := c.deltaWork.Data
		if err = c.manager.Mul(delta, &l.Y, &w[0]); err != nil {
			return err
		}
		if err = c.manager.Add(&l.X, &w[0], &w[1]); err != nil {
			return err
		}
		if err = c.manager.Mul(delta, &u.Y, &w[2]); err != nil {
			return err
		}
		if err = c.manager.Add(&u.X, &w[2], &w[3]); err != nil {
			return err
		}
		cmp, e := c.manager.Compare(&w[1], &w[3])
		if e != nil {
			return e
		}
		if cmp > 0 {
			return ErrAssertion
		}
		if err = c.deltaWork.Close(); err != nil {
			return err
		}
		c.deltaWork = nil
	}
	return nil
}

func (c *LARCoreSolver) FindDeltaForStrictBoxedBounds(out *numeral.Rational) (err error) {
	if c.stopped || c.strictDelta != nil || c.calls.pending() {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	if out == nil {
		return ErrBounds
	}
	c.strictDelta = NewIndexedRationalVector(c.manager, 1)
	if err = c.manager.SetInt32(&c.strictDelta.Data[0], 1); err != nil {
		return err
	}
	for j := uint32(0); j < c.RA.ColumnCount(); j++ {
		if uint64(j) >= uint64(len(c.Bounds.Types.values)) {
			return ErrBounds
		}
		if c.Bounds.Types.values[j] != ColumnBoxed {
			continue
		}
		if uint64(j) >= uint64(len(c.Bounds.Low.values)) || uint64(j) >= uint64(len(c.Bounds.Upper.values)) {
			return ErrBounds
		}
		if err = c.updateDeltaValue(&c.strictDelta.Data[0], &c.Bounds.Low.values[j], &c.Bounds.Upper.values[j]); err != nil {
			return err
		}
	}
	before, e := c.calls.begin(c.manager)
	if e != nil {
		return e
	}
	e = c.manager.Copy(out, &c.strictDelta.Data[0])
	if err = c.calls.after(c.manager, before, e); err != nil {
		return err
	}
	if err = c.strictDelta.Close(); err != nil {
		return err
	}
	c.strictDelta = nil
	return nil
}

func (c *LARCoreSolver) pairBoundToDouble(p *RationalPair, delta float64) (value float64, err error) {
	before, e := c.calls.begin(c.manager)
	if e != nil {
		return 0, e
	}
	defer func() { err = c.calls.after(c.manager, before, err) }()
	x, e := c.manager.Float64(&p.X)
	if e != nil {
		return 0, e
	}
	y, e := c.manager.Float64(&p.Y)
	if e != nil {
		return 0, e
	}
	return x + delta*y, nil
}

func (c *LARCoreSolver) GetBoundsForDoubleSolver() (err error) {
	if c.stopped || c.conversionDelta != nil || c.calls.pending() {
		return ErrExactVectorPending
	}
	defer func() { c.stopped = err != nil }()
	n := c.RA.ColumnCount()
	c.DLow = resizeFloatSolverValues(c.DLow, int(n))
	c.DUpper = resizeFloatSolverValues(c.DUpper, int(n))
	c.conversionDelta = NewIndexedRationalVector(c.manager, 1)
	if err = c.FindDeltaForStrictBoxedBounds(&c.conversionDelta.Data[0]); err != nil {
		return err
	}
	before, e := c.calls.begin(c.manager)
	if e != nil {
		return e
	}
	delta, e := c.manager.Float64(&c.conversionDelta.Data[0])
	if err = c.calls.after(c.manager, before, e); err != nil {
		return err
	}
	if err = c.conversionDelta.Close(); err != nil {
		return err
	}
	c.conversionDelta = nil
	if delta > 0.000001 {
		delta = 0.000001
	}
	for j := uint32(0); j < n; j++ {
		low, e := c.LowBoundIsSet(j)
		if e != nil {
			return e
		}
		if low {
			if uint64(j) >= uint64(len(c.Bounds.Low.values)) {
				return ErrBounds
			}
			value, e := c.pairBoundToDouble(&c.Bounds.Low.values[j], delta)
			if e != nil {
				return e
			}
			c.DLow[j] = value
		}
		upper, e := c.UpperBoundIsSet(j)
		if e != nil {
			return e
		}
		if upper {
			if uint64(j) >= uint64(len(c.Bounds.Upper.values)) {
				return ErrBounds
			}
			value, e := c.pairBoundToDouble(&c.Bounds.Upper.values[j], delta)
			if e != nil {
				return e
			}
			c.DUpper[j] = value
			if sourceDebugEnabled && low && !(c.DUpper[j] >= c.DLow[j]) {
				return ErrAssertion
			}
		}
	}
	return nil
}
