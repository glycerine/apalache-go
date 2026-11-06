package interval

// Source: src/util/ext_numeral.h:27-344, concrete mpfx instantiation reached
// by src/math/subpaving/subpaving_mpfx.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// FixedExtended borrows the subpaving fixed-point numeral manager. Its current
// rounding direction is selected by the concrete caller before an operation.
type FixedExtended struct {
	Numbers *numeral.FixedPointManager
}

// Display writes the concrete subpaving endpoint in source ext_numeral
// spelling. Infinite endpoints never inspect their numeral storage.
func (e FixedExtended) Display(w io.Writer, a *numeral.FixedPoint, k ExtendedKind) error {
	var s string
	switch k {
	case MinusInfinity:
		s = "-oo"
	case PlusInfinity:
		s = "+oo"
	default:
		return e.Numbers.Display(w, a)
	}
	return writeExtendedText(w, s)
}

func (e FixedExtended) IsZero(a *numeral.FixedPoint, k ExtendedKind) (bool, error) {
	if k != Finite {
		return false, nil
	}
	return e.Numbers.IsZero(a)
}

func (e FixedExtended) IsPos(a *numeral.FixedPoint, k ExtendedKind) (bool, error) {
	if k == PlusInfinity {
		return true, nil
	}
	if k == MinusInfinity {
		return false, nil
	}
	return e.Numbers.IsPos(a)
}

func (e FixedExtended) IsNeg(a *numeral.FixedPoint, k ExtendedKind) (bool, error) {
	if k == MinusInfinity {
		return true, nil
	}
	if k == PlusInfinity {
		return false, nil
	}
	return e.Numbers.IsNeg(a)
}

func (e FixedExtended) Compare(a *numeral.FixedPoint, ak ExtendedKind, b *numeral.FixedPoint, bk ExtendedKind) (int, error) {
	if ak != Finite || bk != Finite {
		if ak < bk {
			return -1, nil
		}
		if ak > bk {
			return 1, nil
		}
		return 0, nil
	}
	return e.Numbers.Compare(a, b)
}

func (e FixedExtended) Equal(a *numeral.FixedPoint, ak ExtendedKind, b *numeral.FixedPoint, bk ExtendedKind) (bool, error) {
	if ak != Finite {
		return ak == bk, nil
	}
	if bk != Finite {
		return false, nil
	}
	return e.Numbers.Equal(a, b)
}

func (e FixedExtended) Less(a *numeral.FixedPoint, ak ExtendedKind, b *numeral.FixedPoint, bk ExtendedKind) (bool, error) {
	if ak != Finite || bk != Finite {
		return ak < bk, nil
	}
	return e.Numbers.Less(a, b)
}

func (e FixedExtended) Greater(a *numeral.FixedPoint, ak ExtendedKind, b *numeral.FixedPoint, bk ExtendedKind) (bool, error) {
	return e.Less(b, bk, a, ak)
}

func (e FixedExtended) Neg(a *numeral.FixedPoint, k ExtendedKind) (ExtendedKind, error) {
	if k == MinusInfinity {
		return PlusInfinity, nil
	}
	if k == PlusInfinity {
		return MinusInfinity, nil
	}
	return Finite, e.Numbers.Neg(a)
}

func (e FixedExtended) Add(a *numeral.FixedPoint, ak ExtendedKind, b *numeral.FixedPoint, bk ExtendedKind, out *numeral.FixedPoint) (ExtendedKind, error) {
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

func (e FixedExtended) Sub(a *numeral.FixedPoint, ak ExtendedKind, b *numeral.FixedPoint, bk ExtendedKind, out *numeral.FixedPoint) (ExtendedKind, error) {
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

func (e FixedExtended) Mul(a *numeral.FixedPoint, ak ExtendedKind, b *numeral.FixedPoint, bk ExtendedKind, out *numeral.FixedPoint) (ExtendedKind, error) {
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

func (e FixedExtended) Div(a *numeral.FixedPoint, ak ExtendedKind, b *numeral.FixedPoint, bk ExtendedKind, out *numeral.FixedPoint) (ExtendedKind, error) {
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

func (e FixedExtended) Inv(a *numeral.FixedPoint, k ExtendedKind) (ExtendedKind, error) {
	if k != Finite {
		return Finite, e.Numbers.Reset(a)
	}
	return Finite, e.Numbers.Inv(a)
}

func (e FixedExtended) Power(a *numeral.FixedPoint, k ExtendedKind, exponent uint32) (ExtendedKind, error) {
	if k == MinusInfinity && exponent%2 == 0 {
		return PlusInfinity, nil
	}
	if k != Finite {
		return k, nil
	}
	return Finite, e.Numbers.Power(a, exponent, a)
}
