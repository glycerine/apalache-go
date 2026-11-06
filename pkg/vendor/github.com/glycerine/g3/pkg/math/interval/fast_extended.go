package interval

// Source: src/util/ext_numeral.h:27-344, concrete mpff specialization reached
// by src/math/subpaving/subpaving_mpff.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// FastExtended borrows the concrete subpaving fast-float manager. The caller
// selects its rounding direction before finite arithmetic.
type FastExtended struct{ Numbers *numeral.FastFloatManager }

// Display writes a concrete fast-float endpoint with the source infinity spelling.
func (e FastExtended) Display(w io.Writer, a *numeral.FastFloat, k ExtendedKind) error {
	var s string
	switch k {
	case MinusInfinity:
		s = "-oo"
	case PlusInfinity:
		s = "+oo"
	default:
		return e.Numbers.DisplayTo(w, a)
	}
	return writeExtendedText(w, s)
}

func (e FastExtended) IsZero(a *numeral.FastFloat, k ExtendedKind) (bool, error) {
	if k != Finite {
		return false, nil
	}
	return e.Numbers.IsZero(a)
}

func (e FastExtended) IsPos(a *numeral.FastFloat, k ExtendedKind) (bool, error) {
	if k == PlusInfinity {
		return true, nil
	}
	if k == MinusInfinity {
		return false, nil
	}
	return e.Numbers.IsPos(a)
}

func (e FastExtended) IsNeg(a *numeral.FastFloat, k ExtendedKind) (bool, error) {
	if k == MinusInfinity {
		return true, nil
	}
	if k == PlusInfinity {
		return false, nil
	}
	return e.Numbers.IsNeg(a)
}

func (e FastExtended) Compare(a *numeral.FastFloat, ak ExtendedKind, b *numeral.FastFloat, bk ExtendedKind) (int, error) {
	if ak != Finite || bk != Finite {
		if ak < bk {
			return -1, nil
		}
		if ak > bk {
			return 1, nil
		}
		return 0, nil
	}
	less, err := e.Numbers.Less(a, b)
	if err != nil {
		return 0, err
	}
	if less {
		return -1, nil
	}
	greater, err := e.Numbers.Less(b, a)
	if err != nil {
		return 0, err
	}
	if greater {
		return 1, nil
	}
	return 0, nil
}

func (e FastExtended) Equal(a *numeral.FastFloat, ak ExtendedKind, b *numeral.FastFloat, bk ExtendedKind) (bool, error) {
	if ak != Finite {
		return ak == bk, nil
	}
	if bk != Finite {
		return false, nil
	}
	return e.Numbers.Equal(a, b)
}

func (e FastExtended) Less(a *numeral.FastFloat, ak ExtendedKind, b *numeral.FastFloat, bk ExtendedKind) (bool, error) {
	if ak != Finite || bk != Finite {
		return ak < bk, nil
	}
	return e.Numbers.Less(a, b)
}

func (e FastExtended) Greater(a *numeral.FastFloat, ak ExtendedKind, b *numeral.FastFloat, bk ExtendedKind) (bool, error) {
	return e.Less(b, bk, a, ak)
}

func (e FastExtended) Neg(a *numeral.FastFloat, k ExtendedKind) (ExtendedKind, error) {
	if k == MinusInfinity {
		return PlusInfinity, nil
	}
	if k == PlusInfinity {
		return MinusInfinity, nil
	}
	return Finite, e.Numbers.Neg(a)
}

func (e FastExtended) Add(a *numeral.FastFloat, ak ExtendedKind, b *numeral.FastFloat, bk ExtendedKind, out *numeral.FastFloat) (ExtendedKind, error) {
	if ak != Finite && bk != Finite && ak != bk {
		return Finite, ErrExtendedDomain
	}
	if ak != Finite {
		return ak, e.Numbers.Reset(out)
	}
	if bk != Finite {
		return bk, e.Numbers.Reset(out)
	}
	return Finite, e.Numbers.Add(a, b, out)
}

func (e FastExtended) Sub(a *numeral.FastFloat, ak ExtendedKind, b *numeral.FastFloat, bk ExtendedKind, out *numeral.FastFloat) (ExtendedKind, error) {
	if ak != Finite && ak == bk {
		return Finite, ErrExtendedDomain
	}
	if ak != Finite {
		return ak, e.Numbers.Reset(out)
	}
	if bk == MinusInfinity {
		return PlusInfinity, e.Numbers.Reset(out)
	}
	if bk == PlusInfinity {
		return MinusInfinity, e.Numbers.Reset(out)
	}
	return Finite, e.Numbers.Sub(a, b, out)
}

func (e FastExtended) Mul(a *numeral.FastFloat, ak ExtendedKind, b *numeral.FastFloat, bk ExtendedKind, out *numeral.FastFloat) (ExtendedKind, error) {
	az, err := e.IsZero(a, ak)
	if err != nil {
		return Finite, err
	}
	if az {
		return Finite, e.Numbers.Reset(out)
	}
	bz, err := e.IsZero(b, bk)
	if err != nil {
		return Finite, err
	}
	if bz {
		return Finite, e.Numbers.Reset(out)
	}
	if ak != Finite || bk != Finite {
		ap, err := e.IsPos(a, ak)
		if err != nil {
			return Finite, err
		}
		bp, err := e.IsPos(b, bk)
		if err != nil {
			return Finite, err
		}
		if ap == bp {
			return PlusInfinity, e.Numbers.Reset(out)
		}
		return MinusInfinity, e.Numbers.Reset(out)
	}
	return Finite, e.Numbers.Mul(a, b, out)
}

func (e FastExtended) Div(a *numeral.FastFloat, ak ExtendedKind, b *numeral.FastFloat, bk ExtendedKind, out *numeral.FastFloat) (ExtendedKind, error) {
	bz, err := e.IsZero(b, bk)
	if err != nil {
		return Finite, err
	}
	if bz || ak != Finite && bk != Finite {
		return Finite, ErrExtendedDomain
	}
	az, err := e.IsZero(a, ak)
	if err != nil {
		return Finite, err
	}
	if az || bk != Finite {
		return Finite, e.Numbers.Reset(out)
	}
	if ak != Finite {
		ap, err := e.IsPos(a, ak)
		if err != nil {
			return Finite, err
		}
		bp, err := e.IsPos(b, bk)
		if err != nil {
			return Finite, err
		}
		if ap == bp {
			return PlusInfinity, e.Numbers.Reset(out)
		}
		return MinusInfinity, e.Numbers.Reset(out)
	}
	return Finite, e.Numbers.Div(a, b, out)
}

func (e FastExtended) Inv(a *numeral.FastFloat, k ExtendedKind) (ExtendedKind, error) {
	if k != Finite {
		return Finite, e.Numbers.Reset(a)
	}
	return Finite, e.Numbers.InvInPlace(a)
}

func (e FastExtended) Power(a *numeral.FastFloat, k ExtendedKind, exponent uint32) (ExtendedKind, error) {
	if k == MinusInfinity && exponent%2 == 0 {
		return PlusInfinity, nil
	}
	if k != Finite {
		return k, nil
	}
	return Finite, e.Numbers.Power(a, exponent, a)
}
