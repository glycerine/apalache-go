package interval

// Source: src/math/interval/interval_def.h:173-355, concrete config_mpq.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrRationalRootCanceled = z3exception.NewDefaultException(messages.Canceled)

// RationalRoot borrows the exact numeral manager, resource limit and optional
// process cooperation owner.
type RationalRoot struct {
	Numbers     *numeral.RationalManager
	Limit       *control.Reslimit
	Cooperation *cooperate.Cooperation
	TraceTags   *diagnostic.TraceTags
}

type rationalRootScratch struct {
	m      *numeral.RationalManager
	values []*numeral.Rational
}

func (s *rationalRootScratch) new() *numeral.Rational {
	v := numeral.NewRational()
	s.values = append(s.values, &v)
	return &v
}

func (s *rationalRootScratch) close() (err error) {
	for i := len(s.values) - 1; i >= 0; i-- {
		err = errors.Join(err, s.m.Del(s.values[i]))
	}
	return err
}

func (r RationalRoot) checkpoint() error {
	if !r.Limit.Inc() {
		return ErrRationalRootCanceled
	}
	if r.Cooperation != nil {
		return r.Cooperation.Cooperate("interval")
	}
	return nil
}

// Bounds encloses a^(1/n). Exact rational Newton iterations stop once the
// successive estimates differ by less than the positive precision.
func (r RationalRoot) Bounds(a *numeral.Rational, n uint32, precision, lower, upper *numeral.Rational) (err error) {
	m := r.Numbers
	if m == nil || r.Limit == nil || n == 0 || a == nil || precision == nil || lower == nil || upper == nil || lower == upper {
		return ErrExtendedDomain
	}
	negative, err := m.IsNeg(a)
	if err != nil {
		return err
	}
	if negative && n%2 == 0 {
		return ErrExtendedDomain
	}
	s := rationalRootScratch{m: m}
	defer func() { err = errors.Join(err, s.close()) }()
	lo, hi := s.new(), s.new()
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
		positive, e := m.IsPos(precision)
		if e != nil {
			return e
		}
		if !positive {
			return ErrExtendedDomain
		}
		absolute := s.new()
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

func (r RationalRoot) approx(a *numeral.Rational, n uint32, precision, x *numeral.Rational, s *rationalRootScratch) error {
	m := r.Numbers
	xPrime, d := s.new(), s.new()
	if err := m.SetInt32(d, 1); err != nil {
		return err
	}
	cmp, err := m.Compare(a, d)
	if err != nil {
		return err
	}
	if cmp < 0 {
		if err := m.Copy(x, a); err != nil {
			return err
		}
	} else {
		k, err := m.PrevPowerOfTwo(a)
		if err != nil {
			return err
		}
		if err := m.SetInt32(x, 2); err != nil {
			return err
		}
		if err := m.Power(x, k/n, x); err != nil {
			return err
		}
	}
	if n == 2 {
		two := s.new()
		if err := m.SetInt32(two, 2); err != nil {
			return err
		}
		for {
			if err := r.checkpoint(); err != nil {
				return err
			}
			if err := m.Div(a, x, xPrime); err != nil {
				return err
			}
			if err := m.Add(x, xPrime, xPrime); err != nil {
				return err
			}
			if err := m.Div(xPrime, two, xPrime); err != nil {
				return err
			}
			if err := m.Sub(xPrime, x, d); err != nil {
				return err
			}
			if err := m.Abs(d); err != nil {
				return err
			}
			if err := m.Swap(x, xPrime); err != nil {
				return err
			}
			cmp, err := m.Compare(d, precision)
			if err != nil {
				return err
			}
			if cmp < 0 {
				return nil
			}
		}
	}
	degree, previous := s.new(), s.new()
	if err := m.SetUint32(degree, n); err != nil {
		return err
	}
	if err := m.Copy(previous, degree); err != nil {
		return err
	}
	if err := m.Dec(previous); err != nil {
		return err
	}
	for {
		if err := r.checkpoint(); err != nil {
			return err
		}
		if err := m.Power(x, n-1, xPrime); err != nil {
			return err
		}
		if err := m.Div(a, xPrime, xPrime); err != nil {
			return err
		}
		if err := m.Mul(previous, x, d); err != nil {
			return err
		}
		if err := m.Add(d, xPrime, xPrime); err != nil {
			return err
		}
		if err := m.Div(xPrime, degree, xPrime); err != nil {
			return err
		}
		if err := m.Sub(xPrime, x, d); err != nil {
			return err
		}
		if err := m.Abs(d); err != nil {
			return err
		}
		if err := r.traceNthRootIteration(a, x, xPrime, d); err != nil {
			return err
		}
		if err := m.Swap(x, xPrime); err != nil {
			return err
		}
		cmp, err := m.Compare(d, precision)
		if err != nil {
			return err
		}
		if cmp < 0 {
			return nil
		}
	}
}

func (r RationalRoot) positiveBounds(a *numeral.Rational, n uint32, precision, lo, hi *numeral.Rational, s *rationalRootScratch) error {
	m := r.Numbers
	if err := r.approx(a, n, precision, hi, s); err != nil {
		return err
	}
	if n == 2 {
		if err := m.Div(a, hi, lo); err != nil {
			return err
		}
	} else {
		if err := m.Power(hi, n-1, lo); err != nil {
			return err
		}
		if err := m.Div(a, lo, lo); err != nil {
			return err
		}
	}
	cmp, err := m.Compare(hi, lo)
	if err != nil {
		return err
	}
	if cmp < 0 {
		return m.Swap(lo, hi)
	}
	return nil
}
