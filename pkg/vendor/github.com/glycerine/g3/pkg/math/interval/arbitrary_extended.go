package interval

// Source: src/util/ext_numeral.h:27-344, concrete mpf specialization reached
// by src/math/subpaving/subpaving_mpf.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// ArbitraryExtended borrows the concrete subpaving arbitrary-float manager. The caller
// selects its rounding direction before finite arithmetic.
type ArbitraryExtended struct {
	Numbers *numeral.ArbitraryFloatNumeralManager
}

// Display writes a concrete arbitrary-float endpoint with the source infinity spelling.
func (e ArbitraryExtended) Display(w io.Writer, a *numeral.ArbitraryFloat, k ExtendedKind) error {
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

func (e ArbitraryExtended) IsZero(a *numeral.ArbitraryFloat, k ExtendedKind) (bool, error) {
	if k != Finite {
		return false, nil
	}
	return e.Numbers.IsZero(a)
}

func (e ArbitraryExtended) IsPos(a *numeral.ArbitraryFloat, k ExtendedKind) (bool, error) {
	if k == PlusInfinity {
		return true, nil
	}
	if k == MinusInfinity {
		return false, nil
	}
	return e.Numbers.IsPos(a)
}

func (e ArbitraryExtended) IsNeg(a *numeral.ArbitraryFloat, k ExtendedKind) (bool, error) {
	if k == MinusInfinity {
		return true, nil
	}
	if k == PlusInfinity {
		return false, nil
	}
	return e.Numbers.IsNeg(a)
}

func (e ArbitraryExtended) Compare(a *numeral.ArbitraryFloat, ak ExtendedKind, b *numeral.ArbitraryFloat, bk ExtendedKind) (int, error) {
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

func (e ArbitraryExtended) Equal(a *numeral.ArbitraryFloat, ak ExtendedKind, b *numeral.ArbitraryFloat, bk ExtendedKind) (bool, error) {
	if ak != Finite {
		return ak == bk, nil
	}
	if bk != Finite {
		return false, nil
	}
	return e.Numbers.Equal(a, b)
}

func (e ArbitraryExtended) Less(a *numeral.ArbitraryFloat, ak ExtendedKind, b *numeral.ArbitraryFloat, bk ExtendedKind) (bool, error) {
	if ak != Finite || bk != Finite {
		return ak < bk, nil
	}
	return e.Numbers.Less(a, b)
}

func (e ArbitraryExtended) Greater(a *numeral.ArbitraryFloat, ak ExtendedKind, b *numeral.ArbitraryFloat, bk ExtendedKind) (bool, error) {
	return e.Less(b, bk, a, ak)
}

func (e ArbitraryExtended) Neg(a *numeral.ArbitraryFloat, k ExtendedKind) (ExtendedKind, error) {
	if k == MinusInfinity {
		return PlusInfinity, nil
	}
	if k == PlusInfinity {
		return MinusInfinity, nil
	}
	return Finite, e.Numbers.Neg(a, a)
}

func (e ArbitraryExtended) Add(a *numeral.ArbitraryFloat, ak ExtendedKind, b *numeral.ArbitraryFloat, bk ExtendedKind, out *numeral.ArbitraryFloat) (ExtendedKind, error) {
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

func (e ArbitraryExtended) Sub(a *numeral.ArbitraryFloat, ak ExtendedKind, b *numeral.ArbitraryFloat, bk ExtendedKind, out *numeral.ArbitraryFloat) (ExtendedKind, error) {
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

func (e ArbitraryExtended) Mul(a *numeral.ArbitraryFloat, ak ExtendedKind, b *numeral.ArbitraryFloat, bk ExtendedKind, out *numeral.ArbitraryFloat) (ExtendedKind, error) {
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

func (e ArbitraryExtended) Div(a *numeral.ArbitraryFloat, ak ExtendedKind, b *numeral.ArbitraryFloat, bk ExtendedKind, out *numeral.ArbitraryFloat) (ExtendedKind, error) {
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

func (e ArbitraryExtended) Inv(a *numeral.ArbitraryFloat, k ExtendedKind) (ExtendedKind, error) {
	if k != Finite {
		return Finite, e.Numbers.Reset(a)
	}
	return Finite, e.Numbers.Inv(a)
}

func (e ArbitraryExtended) Power(a *numeral.ArbitraryFloat, k ExtendedKind, exponent uint32) (ExtendedKind, error) {
	if k == MinusInfinity && exponent%2 == 0 {
		return PlusInfinity, nil
	}
	if k != Finite {
		return k, nil
	}
	return Finite, e.Numbers.Power(a, exponent, a)
}
