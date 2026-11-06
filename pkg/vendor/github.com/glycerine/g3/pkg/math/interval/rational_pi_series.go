package interval

// Source: src/math/interval/interval_def.h:1850-1871.

import "github.com/glycerine/g3/pkg/util/numeral"

// piSeries computes the exact BBP term
// 16^-x (4/(8x+1)-2/(8x+4)-1/(8x+5)-1/(8x+6)).
func (r *RationalTranscendentals) piSeries(x uint32, out, term, scale *numeral.Rational) error {
	m := r.Numbers
	if err := m.SetFraction(out, 4, uint64(8*x+1)); err != nil {
		return err
	}
	if err := m.SetFraction(term, 2, uint64(8*x+4)); err != nil {
		return err
	}
	if err := m.Sub(out, term, out); err != nil {
		return err
	}
	if err := m.SetFraction(term, 1, uint64(8*x+5)); err != nil {
		return err
	}
	if err := m.Sub(out, term, out); err != nil {
		return err
	}
	if err := m.SetFraction(term, 1, uint64(8*x+6)); err != nil {
		return err
	}
	if err := m.Sub(out, term, out); err != nil {
		return err
	}
	if err := m.SetFraction(scale, 1, 16); err != nil {
		return err
	}
	if err := m.Power(scale, x, scale); err != nil {
		return err
	}
	return m.Mul(out, scale, out)
}
