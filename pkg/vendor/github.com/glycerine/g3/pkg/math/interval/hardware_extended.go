package interval

// Source: src/util/ext_numeral.h:27-344, concrete f2n<hwf_manager>
// specialization reached by src/math/subpaving/subpaving_hwf.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// HardwareExtended borrows the concrete hardware-float subpaving manager.
// The caller selects rounding before finite arithmetic.
type HardwareExtended struct {
	Numbers *numeral.HardwareFloatNumeralManager
}

func (e HardwareExtended) Display(w io.Writer, a *numeral.HardwareFloat, k ExtendedKind) error {
	switch k {
	case MinusInfinity:
		return writeExtendedText(w, "-oo")
	case PlusInfinity:
		return writeExtendedText(w, "+oo")
	default:
		return e.Numbers.DisplayTo(w, a)
	}
}

func (e HardwareExtended) IsZero(a *numeral.HardwareFloat, k ExtendedKind) bool {
	return k == Finite && e.Numbers.IsZero(a)
}
func (e HardwareExtended) IsPos(a *numeral.HardwareFloat, k ExtendedKind) bool {
	if k == PlusInfinity {
		return true
	}
	if k == MinusInfinity {
		return false
	}
	return e.Numbers.IsPos(a)
}
func (e HardwareExtended) IsNeg(a *numeral.HardwareFloat, k ExtendedKind) bool {
	if k == MinusInfinity {
		return true
	}
	if k == PlusInfinity {
		return false
	}
	return e.Numbers.IsNeg(a)
}
func (e HardwareExtended) Equal(a *numeral.HardwareFloat, ak ExtendedKind, b *numeral.HardwareFloat, bk ExtendedKind) bool {
	if ak != Finite {
		return ak == bk
	}
	if bk != Finite {
		return false
	}
	return e.Numbers.Equal(a, b)
}
func (e HardwareExtended) Less(a *numeral.HardwareFloat, ak ExtendedKind, b *numeral.HardwareFloat, bk ExtendedKind) bool {
	if ak != Finite || bk != Finite {
		return ak < bk
	}
	return e.Numbers.Less(a, b)
}
func (e HardwareExtended) Greater(a *numeral.HardwareFloat, ak ExtendedKind, b *numeral.HardwareFloat, bk ExtendedKind) bool {
	return e.Less(b, bk, a, ak)
}
func (e HardwareExtended) Compare(a *numeral.HardwareFloat, ak ExtendedKind, b *numeral.HardwareFloat, bk ExtendedKind) int {
	if e.Less(a, ak, b, bk) {
		return -1
	}
	if e.Greater(a, ak, b, bk) {
		return 1
	}
	return 0
}
func (e HardwareExtended) Neg(a *numeral.HardwareFloat, k ExtendedKind) (ExtendedKind, error) {
	if k == MinusInfinity {
		return PlusInfinity, nil
	}
	if k == PlusInfinity {
		return MinusInfinity, nil
	}
	return Finite, e.Numbers.Neg(a, a)
}
func (e HardwareExtended) Add(a *numeral.HardwareFloat, ak ExtendedKind, b *numeral.HardwareFloat, bk ExtendedKind, out *numeral.HardwareFloat) (ExtendedKind, error) {
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
func (e HardwareExtended) Sub(a *numeral.HardwareFloat, ak ExtendedKind, b *numeral.HardwareFloat, bk ExtendedKind, out *numeral.HardwareFloat) (ExtendedKind, error) {
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
func (e HardwareExtended) Mul(a *numeral.HardwareFloat, ak ExtendedKind, b *numeral.HardwareFloat, bk ExtendedKind, out *numeral.HardwareFloat) (ExtendedKind, error) {
	if e.IsZero(a, ak) || e.IsZero(b, bk) {
		return Finite, e.Numbers.Reset(out)
	}
	if ak != Finite || bk != Finite {
		if e.IsPos(a, ak) == e.IsPos(b, bk) {
			return PlusInfinity, e.Numbers.Reset(out)
		}
		return MinusInfinity, e.Numbers.Reset(out)
	}
	return Finite, e.Numbers.Mul(a, b, out)
}
func (e HardwareExtended) Div(a *numeral.HardwareFloat, ak ExtendedKind, b *numeral.HardwareFloat, bk ExtendedKind, out *numeral.HardwareFloat) (ExtendedKind, error) {
	if e.IsZero(b, bk) || ak != Finite && bk != Finite {
		return Finite, ErrExtendedDomain
	}
	if e.IsZero(a, ak) || bk != Finite {
		return Finite, e.Numbers.Reset(out)
	}
	if ak != Finite {
		if e.IsPos(a, ak) == e.IsPos(b, bk) {
			return PlusInfinity, e.Numbers.Reset(out)
		}
		return MinusInfinity, e.Numbers.Reset(out)
	}
	return Finite, e.Numbers.Div(a, b, out)
}
func (e HardwareExtended) Inv(a *numeral.HardwareFloat, k ExtendedKind) (ExtendedKind, error) {
	if k != Finite {
		return Finite, e.Numbers.Reset(a)
	}
	return Finite, e.Numbers.Inv(a)
}
func (e HardwareExtended) Power(a *numeral.HardwareFloat, k ExtendedKind, exponent uint32) (ExtendedKind, error) {
	if k == MinusInfinity && exponent%2 == 0 {
		return PlusInfinity, nil
	}
	if k != Finite {
		return k, nil
	}
	return Finite, e.Numbers.Power(a, exponent, a)
}
