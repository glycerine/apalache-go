package interval

// Source: src/math/interval/interval_def.h:175-380, concrete config_hwf.

import (
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/messages"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrHardwareRootCanceled = z3exception.NewDefaultException(messages.Canceled)

// HardwareRoot borrows the concrete f2n<hwf_manager> and checkpoint services.
type HardwareRoot struct {
	Numbers     *numeral.HardwareFloatNumeralManager
	Limit       *control.Reslimit
	Cooperation *cooperate.Cooperation
	TraceTags   *diagnostic.TraceTags
}

func (r HardwareRoot) checkpoint() error {
	if r.Limit != nil && !r.Limit.Inc() {
		return ErrHardwareRootCanceled
	}
	if r.Cooperation != nil {
		return r.Cooperation.Cooperate("interval")
	}
	return nil
}

// Bounds computes directed binary64 bounds on the real nth root. Positive
// precision is required by the source Newton termination test.
func (r HardwareRoot) Bounds(a *numeral.HardwareFloat, n uint32, precision, lower, upper *numeral.HardwareFloat) error {
	m := r.Numbers
	if m == nil || r.Limit == nil || a == nil || precision == nil || lower == nil || upper == nil || lower == upper || n == 0 {
		return ErrExtendedDomain
	}
	if !m.IsPos(precision) || m.IsNeg(a) && n%2 == 0 {
		return ErrExtendedDomain
	}
	var lo, hi numeral.HardwareFloat
	if n == 1 || m.IsZero(a) || m.IsOne(a) || m.IsMinusOne(a) {
		if err := m.Set(&lo, a); err != nil {
			return err
		}
		if err := m.Set(&hi, a); err != nil {
			return err
		}
	} else {
		var absolute numeral.HardwareFloat
		if err := m.Set(&absolute, a); err != nil {
			return err
		}
		if err := m.Abs(&absolute, &absolute); err != nil {
			return err
		}
		if err := r.positiveBounds(&absolute, n, precision, &lo, &hi); err != nil {
			return err
		}
		if err := r.traceNthRoot(&absolute, n, &lo, &hi); err != nil {
			return err
		}
		if m.IsNeg(a) {
			if err := m.Swap(&lo, &hi); err != nil {
				return err
			}
			if err := m.Neg(&lo, &lo); err != nil {
				return err
			}
			if err := m.Neg(&hi, &hi); err != nil {
				return err
			}
		}
	}
	if err := m.Set(lower, &lo); err != nil {
		return err
	}
	return m.Set(upper, &hi)
}

func (r HardwareRoot) aDivXPower(a, x *numeral.HardwareFloat, n uint32, up bool, out *numeral.HardwareFloat) error {
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

func (r HardwareRoot) approx(a *numeral.HardwareFloat, n uint32, precision, x *numeral.HardwareFloat) error {
	m := r.Numbers
	var xPrime, d numeral.HardwareFloat
	if err := m.SetInt32(&d, 1); err != nil {
		return err
	}
	if m.Less(a, &d) {
		if err := m.Set(x, a); err != nil {
			return err
		}
	} else {
		m.SetRounding(false)
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
	m.SetRounding(false)
	if n == 2 {
		var two numeral.HardwareFloat
		if err := m.SetInt32(&two, 2); err != nil {
			return err
		}
		for {
			if err := r.checkpoint(); err != nil {
				return err
			}
			if err := m.Div(a, x, &xPrime); err != nil {
				return err
			}
			if err := m.Add(x, &xPrime, &xPrime); err != nil {
				return err
			}
			if err := m.Div(&xPrime, &two, &xPrime); err != nil {
				return err
			}
			if err := m.Sub(&xPrime, x, &d); err != nil {
				return err
			}
			if err := m.Abs(&d, &d); err != nil {
				return err
			}
			if err := m.Swap(x, &xPrime); err != nil {
				return err
			}
			if m.Less(&d, precision) {
				return nil
			}
		}
	}
	var num, previous numeral.HardwareFloat
	if err := m.SetUint32(&num, n); err != nil {
		return err
	}
	if err := m.Set(&previous, &num); err != nil {
		return err
	}
	if err := m.Dec(&previous); err != nil {
		return err
	}
	for {
		if err := r.checkpoint(); err != nil {
			return err
		}
		if err := m.Power(x, n-1, &xPrime); err != nil {
			return err
		}
		if err := m.Div(a, &xPrime, &xPrime); err != nil {
			return err
		}
		if err := m.Mul(&previous, x, &d); err != nil {
			return err
		}
		if err := m.Add(&d, &xPrime, &xPrime); err != nil {
			return err
		}
		if err := m.Div(&xPrime, &num, &xPrime); err != nil {
			return err
		}
		if err := m.Sub(&xPrime, x, &d); err != nil {
			return err
		}
		if err := m.Abs(&d, &d); err != nil {
			return err
		}
		if err := r.traceNthRootIteration(a, x, &xPrime, &d); err != nil {
			return err
		}
		if err := m.Swap(x, &xPrime); err != nil {
			return err
		}
		if m.Less(&d, precision) {
			return nil
		}
	}
}

func (r HardwareRoot) positiveBounds(a *numeral.HardwareFloat, n uint32, precision, lo, hi *numeral.HardwareFloat) error {
	m := r.Numbers
	if err := r.approx(a, n, precision, hi); err != nil {
		return err
	}
	if err := r.aDivXPower(a, hi, n-1, true, lo); err != nil {
		return err
	}
	if err := r.traceNthRootBoundCheck(a, hi); err != nil {
		return err
	}
	if m.LessEqual(lo, hi) {
		return r.aDivXPower(a, hi, n-1, false, lo)
	}
	if err := m.Swap(lo, hi); err != nil {
		return err
	}
	if err := r.aDivXPower(a, lo, n-1, false, hi); err != nil {
		return err
	}
	if m.LessEqual(lo, hi) {
		return r.aDivXPower(a, lo, n-1, true, hi)
	}
	// The source fallback compares against an uninitialized one. Initialize it
	// to retain the intended conservative [0,1] or [1,A] enclosure.
	var one numeral.HardwareFloat
	if err := m.SetInt32(&one, 1); err != nil {
		return err
	}
	if m.Less(a, &one) {
		if err := m.Reset(lo); err != nil {
			return err
		}
		return m.Set(hi, &one)
	}
	if err := m.Set(lo, &one); err != nil {
		return err
	}
	return m.Set(hi, a)
}
