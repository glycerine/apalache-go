package lp

// Source: src/util/rational.h:449-451,462-464,480-493; reached exact expressions.

import "github.com/glycerine/g3/pkg/util/numeral"

// Source: src/util/rational.h binary operator+ copied-left expression.
func etaRationalSum(m *numeral.RationalManager, temporary, out, left, right *numeral.Rational) error {
	if err := m.Copy(temporary, left); err != nil {
		return err
	}
	if err := m.Add(temporary, right, temporary); err != nil {
		return err
	}
	if err := m.Copy(out, temporary); err != nil {
		return err
	}
	if err := m.Del(temporary); err != nil {
		return err
	}
	*temporary = numeral.NewRational()
	return nil
}

// These two concrete expressions retain rational(left), its compound update,
// and the copy on return from the resulting reference. The matrix owns both
// slots and captures actual numerical invocations around the call. A stopped
// expression leaves both values intact for owning Close; no source replay or
// callback arithmetic adapter is used.
func etaRationalProduct(m *numeral.RationalManager, temporary, out, left, right *numeral.Rational) error {
	if err := m.Copy(temporary, left); err != nil {
		return err
	}
	if err := m.Mul(temporary, right, temporary); err != nil {
		return err
	}
	if err := m.Copy(out, temporary); err != nil {
		return err
	}
	if err := m.Del(temporary); err != nil {
		return err
	}
	*temporary = numeral.NewRational()
	return nil
}
func etaRationalQuotient(m *numeral.RationalManager, temporary, out, left, right *numeral.Rational) error {
	if err := m.Copy(temporary, left); err != nil {
		return err
	}
	if err := m.Div(temporary, right, temporary); err != nil {
		return err
	}
	if err := m.Copy(out, temporary); err != nil {
		return err
	}
	if err := m.Del(temporary); err != nil {
		return err
	}
	*temporary = numeral.NewRational()
	return nil
}

// etaRationalDifference retains rational(left), compound subtraction and the
// return-value copy used by numeric_pair's two-coordinate constructor.
func etaRationalDifference(m *numeral.RationalManager, temporary, out, left, right *numeral.Rational) error {
	if err := m.Copy(temporary, left); err != nil {
		return err
	}
	if err := m.Sub(temporary, right, temporary); err != nil {
		return err
	}
	if err := m.Copy(out, temporary); err != nil {
		return err
	}
	if err := m.Del(temporary); err != nil {
		return err
	}
	*temporary = numeral.NewRational()
	return nil
}
