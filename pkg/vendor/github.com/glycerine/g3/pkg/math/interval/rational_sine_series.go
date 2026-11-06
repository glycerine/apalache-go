package interval

// Source: src/math/interval/interval_def.h:398-435.

import "github.com/glycerine/g3/pkg/util/numeral"

func (r *RationalTranscendentals) sineSeries(a *numeral.Rational, k uint32, out, power, factorial, factor *numeral.Rational) error {
	m := r.Numbers
	if err := m.Copy(out, a); err != nil {
		return err
	}
	subtract := true
	for i := uint32(3); i <= k; i += 2 {
		if err := m.Power(a, i, power); err != nil {
			return err
		}
		if err := rationalFactorial(m, i, factorial, factor); err != nil {
			return err
		}
		if err := m.Div(power, factorial, power); err != nil {
			return err
		}
		var err error
		if subtract {
			err = m.Sub(out, power, out)
		} else {
			err = m.Add(out, power, out)
		}
		if err != nil {
			return err
		}
		subtract = !subtract
	}
	return nil
}
