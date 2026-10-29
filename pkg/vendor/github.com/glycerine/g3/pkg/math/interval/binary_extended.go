package interval

// Source: src/util/ext_numeral.h, concrete mpbq instantiation reached by
// src/math/realclosure/realclosure.cpp:54-135.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// BinaryExtended borrows the real-closure binary rational manager. Division
// uses that consumer's selected precision and directed rounding.
type BinaryExtended struct {
	Numbers           *numeral.BinaryRationalManager
	DivisionPrecision uint32
	RoundUp           bool
}

// Display writes the concrete real-closure endpoint in source ext_numeral
// spelling. Infinite endpoints never inspect their numeral storage.
func (e BinaryExtended) Display(w io.Writer, a *numeral.BinaryRational, k ExtendedKind, pretty bool) error {
	var s string
	switch k {
	case MinusInfinity:
		if pretty {
			s = "-&infin;"
		} else {
			s = "-oo"
		}
	case PlusInfinity:
		if pretty {
			s = "+&infin;"
		} else {
			s = "+oo"
		}
	default:
		if pretty {
			return e.Numbers.DisplayPPTo(w, a)
		}
		return e.Numbers.DisplayTo(w, a)
	}
	return writeExtendedText(w, s)
}

func (e BinaryExtended) IsZero(a *numeral.BinaryRational, k ExtendedKind) (bool, error) {
	if k != Finite {
		return false, nil
	}
	return e.Numbers.IsZero(a)
}

func (e BinaryExtended) IsPos(a *numeral.BinaryRational, k ExtendedKind) (bool, error) {
	if k == PlusInfinity {
		return true, nil
	}
	if k == MinusInfinity {
		return false, nil
	}
	return e.Numbers.IsPos(a)
}

func (e BinaryExtended) IsNeg(a *numeral.BinaryRational, k ExtendedKind) (bool, error) {
	if k == MinusInfinity {
		return true, nil
	}
	if k == PlusInfinity {
		return false, nil
	}
	return e.Numbers.IsNeg(a)
}

func (e BinaryExtended) Compare(a *numeral.BinaryRational, ak ExtendedKind, b *numeral.BinaryRational, bk ExtendedKind) (int, error) {
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
	less, err = e.Numbers.Less(b, a)
	if err != nil {
		return 0, err
	}
	if less {
		return 1, nil
	}
	return 0, nil
}

func (e BinaryExtended) Equal(a *numeral.BinaryRational, ak ExtendedKind, b *numeral.BinaryRational, bk ExtendedKind) (bool, error) {
	if ak != Finite {
		return ak == bk, nil
	}
	if bk != Finite {
		return false, nil
	}
	return e.Numbers.Equal(a, b)
}

func (e BinaryExtended) Less(a *numeral.BinaryRational, ak ExtendedKind, b *numeral.BinaryRational, bk ExtendedKind) (bool, error) {
	if ak != Finite || bk != Finite {
		return ak < bk, nil
	}
	return e.Numbers.Less(a, b)
}

func (e BinaryExtended) Greater(a *numeral.BinaryRational, ak ExtendedKind, b *numeral.BinaryRational, bk ExtendedKind) (bool, error) {
	return e.Less(b, bk, a, ak)
}

func (e BinaryExtended) Add(a *numeral.BinaryRational, ak ExtendedKind, b *numeral.BinaryRational, bk ExtendedKind, out *numeral.BinaryRational) (ExtendedKind, error) {
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

func (e BinaryExtended) Sub(a *numeral.BinaryRational, ak ExtendedKind, b *numeral.BinaryRational, bk ExtendedKind, out *numeral.BinaryRational) (ExtendedKind, error) {
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

func (e BinaryExtended) Mul(a *numeral.BinaryRational, ak ExtendedKind, b *numeral.BinaryRational, bk ExtendedKind, out *numeral.BinaryRational) (ExtendedKind, error) {
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

func (e BinaryExtended) Div(a *numeral.BinaryRational, ak ExtendedKind, b *numeral.BinaryRational, bk ExtendedKind, out *numeral.BinaryRational) (ExtendedKind, error) {
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
	return Finite, e.Numbers.ApproxDiv(a, b, out, e.DivisionPrecision, e.RoundUp)
}

func (e BinaryExtended) Inv(a *numeral.BinaryRational, k ExtendedKind) (kind ExtendedKind, err error) {
	if k != Finite {
		return Finite, e.Numbers.Reset(a)
	}
	z, err := e.Numbers.IsZero(a)
	if err != nil {
		return Finite, err
	}
	if z {
		return Finite, ErrExtendedDomain
	}
	one := numeral.NewBinaryInt(1)
	defer func() { err = errors.Join(err, e.Numbers.Del(&one)) }()
	return Finite, e.Numbers.ApproxDiv(&one, a, a, e.DivisionPrecision, e.RoundUp)
}

func (e BinaryExtended) Power(a *numeral.BinaryRational, k ExtendedKind, n uint32) (ExtendedKind, error) {
	if k == MinusInfinity && n%2 == 0 {
		return PlusInfinity, nil
	}
	if k != Finite {
		return k, nil
	}
	return Finite, e.Numbers.PowerInPlace(a, n)
}
