package interval

// Source: src/math/interval/interval_def.h:175-380, concrete config_mpfx.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrFixedRootCanceled = z3exception.NewDefaultException(messages.Canceled)

// FixedRoot borrows the concrete subpaving numeral manager and resource limit.
// Cooperation is the source's optional single-threaded checkpoint service.
type FixedRoot struct {
	Numbers     *numeral.FixedPointManager
	Limit       *control.Reslimit
	Cooperation *cooperate.Cooperation
	TraceTags   *diagnostic.TraceTags
}

type fixedRootScratch struct {
	m      *numeral.FixedPointManager
	values []*numeral.FixedPoint
}

func (s *fixedRootScratch) new() (*numeral.FixedPoint, error) {
	v, err := s.m.New()
	if err == nil {
		s.values = append(s.values, v)
	}
	return v, err
}

func (s *fixedRootScratch) close() (err error) {
	for i := len(s.values) - 1; i >= 0; i-- {
		err = errors.Join(err, s.values[i].Close())
	}
	return err
}

func (r FixedRoot) checkpoint() error {
	if r.Limit != nil && !r.Limit.Inc() {
		return ErrFixedRootCanceled
	}
	if r.Cooperation != nil {
		return r.Cooperation.Cooperate("interval")
	}
	return nil
}

// Bounds computes the source's lower and upper fixed-point approximations to
// a^(1/n). Positive precision is required by its Newton termination test.
func (r FixedRoot) Bounds(a *numeral.FixedPoint, n uint32, precision, lower, upper *numeral.FixedPoint) (err error) {
	m := r.Numbers
	if m == nil || r.Limit == nil || n == 0 || lower == upper {
		return ErrExtendedDomain
	}
	if _, err = m.Compare(a, precision); err != nil {
		return err
	}
	if _, err = m.Compare(lower, upper); err != nil {
		return err
	}
	pPositive, err := m.IsPos(precision)
	if err != nil {
		return err
	}
	if !pPositive {
		return ErrExtendedDomain
	}
	negative, err := m.IsNeg(a)
	if err != nil {
		return err
	}
	if negative && n%2 == 0 {
		return ErrExtendedDomain
	}
	s := fixedRootScratch{m: m}
	defer func() { err = errors.Join(err, s.close()) }()
	lo, err := s.new()
	if err != nil {
		return err
	}
	hi, err := s.new()
	if err != nil {
		return err
	}
	zero, err := m.IsZero(a)
	if err != nil {
		return err
	}
	one, err := m.IsOne(a)
	if err != nil {
		return err
	}
	minusOne, err := m.IsMinusOne(a)
	if err != nil {
		return err
	}
	if n == 1 || zero || one || minusOne {
		if err = m.Copy(lo, a); err != nil {
			return err
		}
		if err = m.Copy(hi, a); err != nil {
			return err
		}
	} else {
		absolute, e := s.new()
		if e != nil {
			return e
		}
		if err = m.Copy(absolute, a); err != nil {
			return err
		}
		if err = m.Abs(absolute); err != nil {
			return err
		}
		if err = r.positiveBounds(absolute, n, precision, lo, hi, &s); err != nil {
			return err
		}
		if err = r.traceNthRoot(absolute, n, lo, hi); err != nil {
			return err
		}
		if negative {
			if err = m.Swap(lo, hi); err != nil {
				return err
			}
			if err = m.Neg(lo); err != nil {
				return err
			}
			if err = m.Neg(hi); err != nil {
				return err
			}
		}
	}
	if err = m.Copy(lower, lo); err != nil {
		return err
	}
	return m.Copy(upper, hi)
}

func (r FixedRoot) aDivXPower(a, x *numeral.FixedPoint, n uint32, up bool, out *numeral.FixedPoint) error {
	m := r.Numbers
	if n == 1 {
		if err := m.SetRounding(up); err != nil {
			return err
		}
		return m.Div(a, x, out)
	}
	if err := m.SetRounding(!up); err != nil {
		return err
	}
	if err := m.Power(x, n, out); err != nil {
		return err
	}
	if err := m.SetRounding(up); err != nil {
		return err
	}
	return m.Div(a, out, out)
}

func (r FixedRoot) approx(a *numeral.FixedPoint, n uint32, precision, x *numeral.FixedPoint, s *fixedRootScratch) error {
	m := r.Numbers
	xPrime, err := s.new()
	if err != nil {
		return err
	}
	d, err := s.new()
	if err != nil {
		return err
	}
	if err = m.SetInt64(d, 1); err != nil {
		return err
	}
	less, err := m.Less(a, d)
	if err != nil {
		return err
	}
	if less {
		if err = m.Copy(x, a); err != nil {
			return err
		}
	} else {
		if err = m.SetRounding(false); err != nil {
			return err
		}
		k, e := m.PrevPowerOfTwo(a)
		if e != nil {
			return e
		}
		if err = m.SetInt64(x, 2); err != nil {
			return err
		}
		if err = m.Power(x, k/n, x); err != nil {
			return err
		}
	}
	if err = m.SetRounding(false); err != nil {
		return err
	}
	if n == 2 {
		two, e := s.new()
		if e != nil {
			return e
		}
		if err = m.SetInt64(two, 2); err != nil {
			return err
		}
		for {
			if err = r.checkpoint(); err != nil {
				return err
			}
			if err = m.Div(a, x, xPrime); err != nil {
				return err
			}
			if err = m.Add(x, xPrime, xPrime); err != nil {
				return err
			}
			if err = m.Div(xPrime, two, xPrime); err != nil {
				return err
			}
			if err = m.Sub(xPrime, x, d); err != nil {
				return err
			}
			if err = m.Abs(d); err != nil {
				return err
			}
			if err = m.Swap(x, xPrime); err != nil {
				return err
			}
			less, e := m.Less(d, precision)
			if e != nil {
				return e
			}
			if less {
				return nil
			}
		}
	}
	num, err := s.new()
	if err != nil {
		return err
	}
	previous, err := s.new()
	if err != nil {
		return err
	}
	if err = m.SetInt64(num, int64(n)); err != nil {
		return err
	}
	if err = m.Copy(previous, num); err != nil {
		return err
	}
	if err = m.Dec(previous); err != nil {
		return err
	}
	for {
		if err = r.checkpoint(); err != nil {
			return err
		}
		if err = m.Power(x, n-1, xPrime); err != nil {
			return err
		}
		if err = m.Div(a, xPrime, xPrime); err != nil {
			return err
		}
		if err = m.Mul(previous, x, d); err != nil {
			return err
		}
		if err = m.Add(d, xPrime, xPrime); err != nil {
			return err
		}
		if err = m.Div(xPrime, num, xPrime); err != nil {
			return err
		}
		if err = m.Sub(xPrime, x, d); err != nil {
			return err
		}
		if err = m.Abs(d); err != nil {
			return err
		}
		if err = r.traceNthRootIteration(a, x, xPrime, d); err != nil {
			return err
		}
		if err = m.Swap(x, xPrime); err != nil {
			return err
		}
		less, e := m.Less(d, precision)
		if e != nil {
			return e
		}
		if less {
			return nil
		}
	}
}

func (r FixedRoot) positiveBounds(a *numeral.FixedPoint, n uint32, precision, lo, hi *numeral.FixedPoint, s *fixedRootScratch) error {
	m := r.Numbers
	if err := r.approx(a, n, precision, hi, s); err != nil {
		return err
	}
	if err := r.aDivXPower(a, hi, n-1, true, lo); err != nil {
		return err
	}
	if err := r.traceNthRootBoundCheck(a, hi); err != nil {
		return err
	}
	cmp, err := m.Compare(lo, hi)
	if err != nil {
		return err
	}
	if cmp <= 0 {
		return r.aDivXPower(a, hi, n-1, false, lo)
	}
	if err = m.Swap(lo, hi); err != nil {
		return err
	}
	if err = r.aDivXPower(a, lo, n-1, false, hi); err != nil {
		return err
	}
	cmp, err = m.Compare(lo, hi)
	if err != nil {
		return err
	}
	if cmp <= 0 {
		return r.aDivXPower(a, lo, n-1, true, hi)
	}
	// The source's conservative fallback declares `one` but does not set it
	// to one before the comparison. Correct that initialization so A < 1
	// yields the stated [0,1] enclosure instead of reversed [1,A].
	one, err := s.new()
	if err != nil {
		return err
	}
	if err = m.SetInt64(one, 1); err != nil {
		return err
	}
	less, err := m.Less(a, one)
	if err != nil {
		return err
	}
	if less {
		if err = m.Reset(lo); err != nil {
			return err
		}
		return m.Copy(hi, one)
	}
	if err = m.Copy(lo, one); err != nil {
		return err
	}
	return m.Copy(hi, a)
}
