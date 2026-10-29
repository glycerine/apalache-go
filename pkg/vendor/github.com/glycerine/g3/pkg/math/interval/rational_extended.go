package interval

// Source: src/util/ext_numeral.h, concrete mpq instantiation reached by
// src/math/interval/interval.h and interval_def.h.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// ExtendedKind is the source ordering -infinity, finite, +infinity.
type ExtendedKind uint8

const (
	MinusInfinity ExtendedKind = iota
	Finite
	PlusInfinity
)

var ErrExtendedDomain = errors.New("interval: undefined extended rational operation")

// RationalExtended borrows the rational manager. Infinity operations reset
// their output numeral, as the source does, even though its kind is infinite.
type RationalExtended struct{ Numbers *numeral.RationalManager }

// Display writes the retained ext_numeral endpoint spelling. Infinite kinds
// do not inspect the numeral, whose storage is reset by the interval owner.
func (e RationalExtended) Display(w io.Writer, a *numeral.Rational, k ExtendedKind) error {
	switch k {
	case MinusInfinity:
		return writeExtendedText(w, "-oo")
	case PlusInfinity:
		return writeExtendedText(w, "+oo")
	default:
		return e.Numbers.DisplayTo(w, a)
	}
}

func (e RationalExtended) IsZero(a *numeral.Rational, k ExtendedKind) (bool, error) {
	if k != Finite {
		return false, nil
	}
	return e.Numbers.IsZero(a)
}

func (e RationalExtended) IsPos(a *numeral.Rational, k ExtendedKind) (bool, error) {
	if k == PlusInfinity {
		return true, nil
	}
	if k == MinusInfinity {
		return false, nil
	}
	return e.Numbers.IsPos(a)
}

func (e RationalExtended) IsNeg(a *numeral.Rational, k ExtendedKind) (bool, error) {
	if k == MinusInfinity {
		return true, nil
	}
	if k == PlusInfinity {
		return false, nil
	}
	return e.Numbers.IsNeg(a)
}

func (e RationalExtended) Compare(a *numeral.Rational, ak ExtendedKind, b *numeral.Rational, bk ExtendedKind) (int, error) {
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

// Equal follows ext_numeral::eq: finite values use the manager's structural
// equality, while infinite kinds do not read either numeral.
func (e RationalExtended) Equal(a *numeral.Rational, ak ExtendedKind, b *numeral.Rational, bk ExtendedKind) (bool, error) {
	if ak != Finite {
		return ak == bk, nil
	}
	if bk != Finite {
		return false, nil
	}
	return e.Numbers.Equal(a, b)
}

func (e RationalExtended) Less(a *numeral.Rational, ak ExtendedKind, b *numeral.Rational, bk ExtendedKind) (bool, error) {
	if ak != Finite || bk != Finite {
		return ak < bk, nil
	}
	cmp, err := e.Numbers.Compare(a, b)
	return cmp < 0, err
}

func (e RationalExtended) Greater(a *numeral.Rational, ak ExtendedKind, b *numeral.Rational, bk ExtendedKind) (bool, error) {
	return e.Less(b, bk, a, ak)
}

func (e RationalExtended) Add(a *numeral.Rational, ak ExtendedKind, b *numeral.Rational, bk ExtendedKind, out *numeral.Rational) (ExtendedKind, error) {
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

func (e RationalExtended) Sub(a *numeral.Rational, ak ExtendedKind, b *numeral.Rational, bk ExtendedKind, out *numeral.Rational) (ExtendedKind, error) {
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

func (e RationalExtended) Mul(a *numeral.Rational, ak ExtendedKind, b *numeral.Rational, bk ExtendedKind, out *numeral.Rational) (ExtendedKind, error) {
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

func (e RationalExtended) Div(a *numeral.Rational, ak ExtendedKind, b *numeral.Rational, bk ExtendedKind, out *numeral.Rational) (ExtendedKind, error) {
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

func (e RationalExtended) Inv(a *numeral.Rational, k ExtendedKind) (ExtendedKind, error) {
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
	return Finite, e.Numbers.InvInPlace(a)
}

func (e RationalExtended) Power(a *numeral.Rational, k ExtendedKind, n uint32) (ExtendedKind, error) {
	if k == MinusInfinity && n%2 == 0 {
		return PlusInfinity, nil
	}
	if k != Finite {
		return k, nil
	}
	return Finite, e.Numbers.Power(a, n, a)
}
