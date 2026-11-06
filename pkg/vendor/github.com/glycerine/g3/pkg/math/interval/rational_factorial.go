package interval

// Source: src/math/interval/interval_def.h:384-396.

import "github.com/glycerine/g3/pkg/util/numeral"

func rationalFactorial(m *numeral.RationalManager, n uint32, out, factor *numeral.Rational) error {
	if err := m.SetInt32(out, 1); err != nil {
		return err
	}
	for i := uint32(2); i <= n; i++ {
		if err := m.SetUint32(factor, i); err != nil {
			return err
		}
		if err := m.Mul(factor, out, out); err != nil {
			return err
		}
	}
	return nil
}
