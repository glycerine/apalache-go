package interval

// Source: src/math/interval/interval_def.h:175-380, concrete config_mpff.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrFastRootCanceled = z3exception.NewDefaultException(messages.Canceled)

// FastRoot borrows the concrete subpaving numeral manager and resource limit.
// Cooperation is the source's optional single-threaded checkpoint service.
type FastRoot struct {
	Numbers     *numeral.FastFloatManager
	Limit       *control.Reslimit
	Cooperation *cooperate.Cooperation
	TraceTags   *diagnostic.TraceTags
}

type fastRootScratch struct {
	m      *numeral.FastFloatManager
	owners []*numeral.ScopedFastFloat
}

func fastRootCompare(m *numeral.FastFloatManager, a, b *numeral.FastFloat) (int, error) {
	less, err := m.Less(a, b)
	if err != nil {
		return 0, err
	}
	if less {
		return -1, nil
	}
	greater, err := m.Less(b, a)
	if err != nil {
		return 0, err
	}
	if greater {
		return 1, nil
	}
	return 0, nil
}

func (s *fastRootScratch) new() (*numeral.FastFloat, error) {
	owner, err := numeral.NewScopedFastFloat(s.m)
	if err != nil {
		return nil, err
	}
	s.owners = append(s.owners, owner)
	return owner.Get()
}

func (s *fastRootScratch) close() (err error) {
	for i := len(s.owners) - 1; i >= 0; i-- {
		err = errors.Join(err, s.owners[i].Close())
	}
	return err
}

func (r FastRoot) checkpoint() error {
	if r.Limit != nil && !r.Limit.Inc() {
		return ErrFastRootCanceled
	}
	if r.Cooperation != nil {
		return r.Cooperation.Cooperate("interval")
	}
	return nil
}

// Bounds computes the source's lower and upper fast-float approximations to
// a^(1/n). Positive precision is required by its Newton termination test.
func (r FastRoot) Bounds(a *numeral.FastFloat, n uint32, precision, lower, upper *numeral.FastFloat) (err error) {
	m := r.Numbers
	if m == nil || r.Limit == nil || n == 0 || lower == upper {
		return ErrExtendedDomain
	}
	if _, err = fastRootCompare(m, a, precision); err != nil {
		return err
	}
	if _, err = fastRootCompare(m, lower, upper); err != nil {
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
	s := fastRootScratch{m: m}
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

func (r FastRoot) aDivXPower(a, x *numeral.FastFloat, n uint32, up bool, out *numeral.FastFloat) error {
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

func (r FastRoot) approx(a *numeral.FastFloat, n uint32, precision, x *numeral.FastFloat, s *fastRootScratch) error {
	m := r.Numbers
	xPrime, err := s.new()
	if err != nil {
		return err
	}
	d, err := s.new()
	if err != nil {
		return err
	}
	if err = m.SetInt32(d, 1); err != nil {
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
		if err = m.SetInt32(x, 2); err != nil {
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
		if err = m.SetInt32(two, 2); err != nil {
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
	if err = m.SetUint32(num, n); err != nil {
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

func (r FastRoot) positiveBounds(a *numeral.FastFloat, n uint32, precision, lo, hi *numeral.FastFloat, s *fastRootScratch) error {
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
	cmp, err := fastRootCompare(m, lo, hi)
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
	cmp, err = fastRootCompare(m, lo, hi)
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
	if err = m.SetInt32(one, 1); err != nil {
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
