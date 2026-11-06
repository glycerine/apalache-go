package interval

// Source: src/math/interval/interval_def.h:175-380, concrete config_mpf.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrArbitraryRootCanceled = z3exception.NewDefaultException(messages.Canceled)

// ArbitraryRoot borrows the concrete subpaving numeral manager and resource limit.
// Cooperation is the source's optional single-threaded checkpoint service.
type ArbitraryRoot struct {
	Numbers     *numeral.ArbitraryFloatNumeralManager
	Limit       *control.Reslimit
	Cooperation *cooperate.Cooperation
	TraceTags   *diagnostic.TraceTags
}

type arbitraryRootScratch struct {
	m           *numeral.ArbitraryFloatNumeralManager
	owners      []*numeral.ScopedArbitraryFloatNumeral
	cursor      int
	closing     bool
	closed      bool
	sourceCause error
}

// ArbitraryRootCleanupError retains the exact scoped-numeral suffix when a
// source destructor stops. Cleanup resumes it with owning deletion.
type ArbitraryRootCleanupError struct {
	Cause   error
	scratch *arbitraryRootScratch
}

func (e *ArbitraryRootCleanupError) Error() string { return e.Cause.Error() }
func (e *ArbitraryRootCleanupError) Unwrap() error { return e.Cause }
func (e *ArbitraryRootCleanupError) Cleanup() error {
	if e == nil || e.scratch == nil {
		return nil
	}
	s := e.scratch
	for s.cursor >= 0 {
		if err := s.owners[s.cursor].Cleanup(); err != nil {
			return err
		}
		s.cursor--
	}
	s.owners = nil
	s.closed = true
	e.scratch = nil
	return nil
}

func arbitraryRootCompare(m *numeral.ArbitraryFloatNumeralManager, a, b *numeral.ArbitraryFloat) (int, error) {
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

func (s *arbitraryRootScratch) new() (*numeral.ArbitraryFloat, error) {
	owner, err := numeral.NewScopedArbitraryFloatNumeral(s.m)
	if err != nil {
		return nil, err
	}
	s.owners = append(s.owners, owner)
	return owner.Get()
}

func (s *arbitraryRootScratch) close() (err error) {
	if s.closed {
		return nil
	}
	if s.sourceCause != nil {
		return &ArbitraryRootCleanupError{Cause: s.sourceCause, scratch: s}
	}
	if !s.closing {
		s.cursor = len(s.owners) - 1
		s.closing = true
	}
	for s.cursor >= 0 {
		if e := s.owners[s.cursor].Close(); e != nil {
			s.sourceCause = e
			return &ArbitraryRootCleanupError{Cause: e, scratch: s}
		}
		s.cursor--
	}
	s.owners = nil
	s.closed = true
	return nil
}

func (r ArbitraryRoot) checkpoint() error {
	if r.Limit != nil && !r.Limit.Inc() {
		return ErrArbitraryRootCanceled
	}
	if r.Cooperation != nil {
		return r.Cooperation.Cooperate("interval")
	}
	return nil
}

// Bounds computes the source's lower and upper arbitrary-float approximations to
// a^(1/n). Positive precision is required by its Newton termination test.
func (r ArbitraryRoot) Bounds(a *numeral.ArbitraryFloat, n uint32, precision, lower, upper *numeral.ArbitraryFloat) (err error) {
	m := r.Numbers
	if m == nil || r.Limit == nil || n == 0 || lower == nil || upper == nil || lower == upper {
		return ErrExtendedDomain
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
	s := arbitraryRootScratch{m: m}
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
		if err = m.Abs(absolute, absolute); err != nil {
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
			if err = m.Neg(lo, lo); err != nil {
				return err
			}
			if err = m.Neg(hi, hi); err != nil {
				return err
			}
		}
	}
	if err = m.Copy(lower, lo); err != nil {
		return err
	}
	if err = m.Copy(upper, hi); err != nil {
		return err
	}
	return nil
}

func (r ArbitraryRoot) aDivXPower(a, x *numeral.ArbitraryFloat, n uint32, up bool, out *numeral.ArbitraryFloat) error {
	m := r.Numbers
	if n == 1 {
		m.SetRounding(up)
		return m.Div(a, x, out)
	}
	m.SetRounding(!up)
	if err := m.Power(x, n, out); err != nil {
		return err
	}
	m.SetRounding(up)
	return m.Div(a, out, out)
}

func (r ArbitraryRoot) approx(a *numeral.ArbitraryFloat, n uint32, precision, x *numeral.ArbitraryFloat, s *arbitraryRootScratch) error {
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
		m.SetRounding(false)
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
	m.SetRounding(false)
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
			if err = m.Abs(d, d); err != nil {
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
	if err = m.SetUnsigned(num, n); err != nil {
		return err
	}
	if err = m.Copy(previous, num); err != nil {
		return err
	}
	if err = m.Decrement(previous); err != nil {
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
		if err = m.Abs(d, d); err != nil {
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

func (r ArbitraryRoot) positiveBounds(a *numeral.ArbitraryFloat, n uint32, precision, lo, hi *numeral.ArbitraryFloat, s *arbitraryRootScratch) error {
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
	cmp, err := arbitraryRootCompare(m, lo, hi)
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
	cmp, err = arbitraryRootCompare(m, lo, hi)
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
