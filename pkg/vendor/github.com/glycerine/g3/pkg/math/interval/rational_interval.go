package interval

// Source: src/math/interval/interval.h:32-67,137-190 and
// src/math/interval/interval_def.h:597-709,795-837.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// RationalInterval owns its two rational endpoints and borrows their manager.
// Infinite endpoints retain reset numeral storage, matching interval_manager.
type RationalInterval struct {
	self                 *RationalInterval
	numbers              *numeral.RationalManager
	lower, upper         numeral.Rational
	lowerOpen, upperOpen bool
	lowerInf, upperInf   bool
	closed               bool
}

func NewRationalInterval(m *numeral.RationalManager) (*RationalInterval, error) {
	if m == nil {
		return nil, arena.ErrInvalidSlot
	}
	v := &RationalInterval{numbers: m, lower: numeral.NewRational(), upper: numeral.NewRational()}
	v.self = v
	return v, nil
}

func (v *RationalInterval) open() error {
	if v == nil || v.self != v || v.numbers == nil {
		return arena.ErrInvalidSlot
	}
	if v.closed {
		return arena.ErrClosed
	}
	return nil
}

func (v *RationalInterval) lowerKind() ExtendedKind {
	if v.lowerInf {
		return MinusInfinity
	}
	return Finite
}

func (v *RationalInterval) upperKind() ExtendedKind {
	if v.upperInf {
		return PlusInfinity
	}
	return Finite
}

func (v *RationalInterval) SetFinite(lower, upper *numeral.Rational, lowerOpen, upperOpen bool) error {
	if err := v.open(); err != nil {
		return err
	}
	e := RationalExtended{Numbers: v.numbers}
	equal, err := e.Equal(lower, Finite, upper, Finite)
	if err != nil {
		return err
	}
	if equal {
		if lowerOpen || upperOpen {
			return ErrExtendedDomain
		}
	} else {
		less, err := e.Less(lower, Finite, upper, Finite)
		if err != nil {
			return err
		}
		if !less {
			return ErrExtendedDomain
		}
	}
	if err = v.numbers.Copy(&v.lower, lower); err != nil {
		return err
	}
	if err = v.numbers.Copy(&v.upper, upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = false, false
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return nil
}

func (v *RationalInterval) Reset() error {
	if err := v.open(); err != nil {
		return err
	}
	if err := v.ResetLower(); err != nil {
		return err
	}
	return v.ResetUpper()
}

func (v *RationalInterval) ResetLower() error {
	if err := v.open(); err != nil {
		return err
	}
	if err := v.numbers.Reset(&v.lower); err != nil {
		return err
	}
	v.lowerInf, v.lowerOpen = true, true
	return nil
}

func (v *RationalInterval) ResetUpper() error {
	if err := v.open(); err != nil {
		return err
	}
	if err := v.numbers.Reset(&v.upper); err != nil {
		return err
	}
	v.upperInf, v.upperOpen = true, true
	return nil
}

func (v *RationalInterval) CheckInvariant() error {
	if err := v.open(); err != nil {
		return err
	}
	e := RationalExtended{Numbers: v.numbers}
	equal, err := e.Equal(&v.lower, v.lowerKind(), &v.upper, v.upperKind())
	if err != nil {
		return err
	}
	if equal {
		if v.lowerOpen || v.upperOpen {
			return ErrExtendedDomain
		}
		return nil
	}
	less, err := e.Less(&v.lower, v.lowerKind(), &v.upper, v.upperKind())
	if err != nil {
		return err
	}
	if !less {
		return ErrExtendedDomain
	}
	return nil
}

func (v *RationalInterval) ContainsZero() (bool, error) {
	if err := v.open(); err != nil {
		return false, err
	}
	e := RationalExtended{Numbers: v.numbers}
	neg, err := e.IsNeg(&v.lower, v.lowerKind())
	if err != nil {
		return false, err
	}
	zero, err := e.IsZero(&v.lower, v.lowerKind())
	if err != nil {
		return false, err
	}
	if !neg && (!zero || v.lowerOpen) {
		return false, nil
	}
	pos, err := e.IsPos(&v.upper, v.upperKind())
	if err != nil {
		return false, err
	}
	zero, err = e.IsZero(&v.upper, v.upperKind())
	if err != nil {
		return false, err
	}
	return pos || zero && !v.upperOpen, nil
}

func (v *RationalInterval) Contains(value *numeral.Rational) (bool, error) {
	if err := v.open(); err != nil {
		return false, err
	}
	e := RationalExtended{Numbers: v.numbers}
	if !v.lowerInf {
		less, err := e.Less(value, Finite, &v.lower, Finite)
		if err != nil {
			return false, err
		}
		if less {
			return false, nil
		}
		if v.lowerOpen {
			equal, err := v.numbers.Equal(value, &v.lower)
			if err != nil {
				return false, err
			}
			if equal {
				return false, nil
			}
		}
	}
	if !v.upperInf {
		greater, err := e.Less(&v.upper, Finite, value, Finite)
		if err != nil {
			return false, err
		}
		if greater {
			return false, nil
		}
		if v.upperOpen {
			equal, err := v.numbers.Equal(value, &v.upper)
			if err != nil {
				return false, err
			}
			if equal {
				return false, nil
			}
		}
	}
	return true, nil
}

func (v *RationalInterval) Equal(other *RationalInterval) (bool, error) {
	if err := v.open(); err != nil {
		return false, err
	}
	if err := other.open(); err != nil {
		return false, err
	}
	if v.numbers != other.numbers {
		return false, arena.ErrInvalidSlot
	}
	e := RationalExtended{Numbers: v.numbers}
	l, err := e.Equal(&v.lower, v.lowerKind(), &other.lower, other.lowerKind())
	if err != nil {
		return false, err
	}
	if !l {
		return false, nil
	}
	u, err := e.Equal(&v.upper, v.upperKind(), &other.upper, other.upperKind())
	if err != nil {
		return false, err
	}
	return u && v.lowerOpen == other.lowerOpen && v.upperOpen == other.upperOpen, nil
}

func (v *RationalInterval) Before(other *RationalInterval) (bool, error) {
	if err := v.open(); err != nil {
		return false, err
	}
	if err := other.open(); err != nil {
		return false, err
	}
	if v.numbers != other.numbers {
		return false, arena.ErrInvalidSlot
	}
	if v.upperInf || other.lowerInf {
		return false, nil
	}
	e := RationalExtended{Numbers: v.numbers}
	less, err := e.Less(&v.upper, Finite, &other.lower, Finite)
	if err != nil {
		return false, err
	}
	if less {
		return true, nil
	}
	if !v.upperOpen {
		return false, nil
	}
	return v.numbers.Equal(&v.upper, &other.lower)
}

func (v *RationalInterval) Add(a, b *RationalInterval) error {
	if err := v.binaryOperands(a, b); err != nil {
		return err
	}
	e := RationalExtended{Numbers: v.numbers}
	lk, err := e.Add(&a.lower, a.lowerKind(), &b.lower, b.lowerKind(), &v.lower)
	if err != nil {
		return err
	}
	uk, err := e.Add(&a.upper, a.upperKind(), &b.upper, b.upperKind(), &v.upper)
	if err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == MinusInfinity, uk == PlusInfinity
	v.lowerOpen, v.upperOpen = a.lowerOpen || b.lowerOpen, a.upperOpen || b.upperOpen
	return v.CheckInvariant()
}

func (v *RationalInterval) Sub(a, b *RationalInterval) error {
	if err := v.binaryOperands(a, b); err != nil {
		return err
	}
	e := RationalExtended{Numbers: v.numbers}
	lk, err := e.Sub(&a.lower, a.lowerKind(), &b.upper, b.upperKind(), &v.lower)
	if err != nil {
		return err
	}
	uk, err := e.Sub(&a.upper, a.upperKind(), &b.lower, b.lowerKind(), &v.upper)
	if err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == MinusInfinity, uk == PlusInfinity
	v.lowerOpen, v.upperOpen = a.lowerOpen || b.upperOpen, a.upperOpen || b.lowerOpen
	return v.CheckInvariant()
}

// Neg follows the source endpoint swap, including the in-place finite case.
func (v *RationalInterval) Neg(a *RationalInterval) error {
	if err := v.open(); err != nil {
		return err
	}
	if err := a.open(); err != nil {
		return err
	}
	if v.numbers != a.numbers {
		return arena.ErrInvalidSlot
	}
	m := v.numbers
	lo, uo := a.lowerOpen, a.upperOpen
	if a.lowerInf {
		if a.upperInf {
			return v.Reset()
		}
		if err := m.Copy(&v.lower, &a.upper); err != nil {
			return err
		}
		if err := m.Neg(&v.lower); err != nil {
			return err
		}
		if err := m.Reset(&v.upper); err != nil {
			return err
		}
		v.lowerInf, v.upperInf = false, true
		v.lowerOpen, v.upperOpen = uo, true
	} else if a.upperInf {
		if err := m.Copy(&v.upper, &a.lower); err != nil {
			return err
		}
		if err := m.Neg(&v.upper); err != nil {
			return err
		}
		if err := m.Reset(&v.lower); err != nil {
			return err
		}
		v.lowerInf, v.upperInf = true, false
		v.lowerOpen, v.upperOpen = true, lo
	} else {
		if v == a {
			if err := m.Swap(&v.lower, &v.upper); err != nil {
				return err
			}
		} else {
			if err := m.Copy(&v.lower, &a.upper); err != nil {
				return err
			}
			if err := m.Copy(&v.upper, &a.lower); err != nil {
				return err
			}
		}
		if err := m.Neg(&v.lower); err != nil {
			return err
		}
		if err := m.Neg(&v.upper); err != nil {
			return err
		}
		v.lowerInf, v.upperInf = false, false
		v.lowerOpen, v.upperOpen = uo, lo
	}
	return v.CheckInvariant()
}

// MulScalar and DivScalar preserve the source div_mul zero-scalar branch:
// it resets the destination to the unbounded interval.
func (v *RationalInterval) MulScalar(k *numeral.Rational, a *RationalInterval) error {
	return v.divMul(k, a, false)
}

func (v *RationalInterval) DivScalar(a *RationalInterval, k *numeral.Rational) error {
	return v.divMul(k, a, true)
}

func (v *RationalInterval) divMul(k *numeral.Rational, a *RationalInterval, inverse bool) (err error) {
	if err = v.open(); err != nil {
		return err
	}
	if err = a.open(); err != nil {
		return err
	}
	if v.numbers != a.numbers {
		return arena.ErrInvalidSlot
	}
	m := v.numbers
	z, err := m.IsZero(k)
	if err != nil {
		return err
	}
	if z {
		return v.Reset()
	}
	positive, err := m.IsPos(k)
	if err != nil {
		return err
	}
	// The source uses manager-owned scratch numerals and swaps them with the
	// destination. Local owned numerals have the same transfer and cleanup.
	lower, upper := numeral.NewRational(), numeral.NewRational()
	defer func() { err = errors.Join(err, m.Del(&lower), m.Del(&upper)) }()
	factor := k
	if inverse {
		inv := numeral.NewRational()
		defer func() { err = errors.Join(err, m.Del(&inv)) }()
		if err = m.Inv(k, &inv); err != nil {
			return err
		}
		factor = &inv
	}
	e := RationalExtended{Numbers: m}
	var lk, uk ExtendedKind
	if positive {
		lk, err = e.Mul(&a.lower, a.lowerKind(), factor, Finite, &lower)
		if err != nil {
			return err
		}
		uk, err = e.Mul(&a.upper, a.upperKind(), factor, Finite, &upper)
		if err != nil {
			return err
		}
	} else {
		lk, err = e.Mul(&a.upper, a.upperKind(), factor, Finite, &lower)
		if err != nil {
			return err
		}
		uk, err = e.Mul(&a.lower, a.lowerKind(), factor, Finite, &upper)
		if err != nil {
			return err
		}
	}
	lo, uo := a.lowerOpen, a.upperOpen
	if err = m.Swap(&v.lower, &lower); err != nil {
		return err
	}
	if err = m.Swap(&v.upper, &upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = lk == MinusInfinity, uk == PlusInfinity
	if positive {
		v.lowerOpen, v.upperOpen = lo, uo
	} else {
		v.lowerOpen, v.upperOpen = uo, lo
	}
	return v.CheckInvariant()
}

func (v *RationalInterval) binaryOperands(a, b *RationalInterval) error {
	if err := v.open(); err != nil {
		return err
	}
	if err := a.open(); err != nil {
		return err
	}
	if err := b.open(); err != nil {
		return err
	}
	if v.numbers != a.numbers || v.numbers != b.numbers {
		return arena.ErrInvalidSlot
	}
	return nil
}

func (v *RationalInterval) Display(w io.Writer) error {
	if err := v.open(); err != nil {
		return err
	}
	left, right := "[", "]"
	if v.lowerOpen {
		left = "("
	}
	if v.upperOpen {
		right = ")"
	}
	if err := writeExtendedText(w, left); err != nil {
		return err
	}
	e := RationalExtended{Numbers: v.numbers}
	if err := e.Display(w, &v.lower, v.lowerKind()); err != nil {
		return err
	}
	if err := writeExtendedText(w, ", "); err != nil {
		return err
	}
	if err := e.Display(w, &v.upper, v.upperKind()); err != nil {
		return err
	}
	return writeExtendedText(w, right)
}

func (v *RationalInterval) Close() error {
	if v == nil || v.self != v {
		return arena.ErrInvalidSlot
	}
	if v.closed {
		return nil
	}
	if err := errors.Join(v.numbers.Del(&v.lower), v.numbers.Del(&v.upper)); err != nil {
		return err
	}
	v.closed = true
	return nil
}
