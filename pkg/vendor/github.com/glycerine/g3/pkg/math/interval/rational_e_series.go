package interval

// Source: src/math/interval/interval_def.h:1945-1959.

import "github.com/glycerine/g3/pkg/util/numeral"

func (r *RationalTranscendentals) eSeries(k uint32, out, denominator, term *numeral.Rational) error {
	m := r.Numbers
	if err := m.SetInt32(out, 2); err != nil {
		return err
	}
	if err := m.SetInt32(denominator, 1); err != nil {
		return err
	}
	for i := uint32(2); i <= k; i++ {
		if err := m.SetUint32(term, i); err != nil {
			return err
		}
		if err := m.Mul(denominator, term, denominator); err != nil {
			return err
		}
		if err := m.Inv(denominator, term); err != nil {
			return err
		}
		if err := m.Add(out, term, out); err != nil {
			return err
		}
	}
	return nil
}
