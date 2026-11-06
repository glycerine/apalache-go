package subpaving

// Source: src/math/subpaving/subpaving_t.h:239-316 and
// src/math/interval/interval_def.h:597-709, concrete config_mpq instance.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// RationalInterval owns mutable endpoint numerals or borrows a live node view.
type RationalInterval struct {
	self                 *RationalInterval
	numbers              *numeral.RationalManager
	lower, upper         numeral.Rational
	constantNode         *RationalNode
	constantVariable     uint32
	lowerInf, upperInf   bool
	lowerOpen, upperOpen bool
	mutableSnapshot      bool
	savedLowerInf        bool
	savedUpperInf        bool
	savedLowerOpen       bool
	savedUpperOpen       bool
	closed               bool
}

func NewRationalInterval(m *numeral.RationalManager) (*RationalInterval, error) {
	if m == nil {
		return nil, arena.ErrInvalidSlot
	}
	v := &RationalInterval{
		numbers: m, lower: numeral.NewRational(), upper: numeral.NewRational(),
		lowerInf: true, upperInf: true, lowerOpen: true, upperOpen: true,
	}
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

func (v *RationalInterval) resolved() (*numeral.Rational, *numeral.Rational, error) {
	if err := v.open(); err != nil {
		return nil, nil, err
	}
	lower, upper := &v.lower, &v.upper
	if v.constantNode != nil {
		lo, hi, err := v.constantNode.Bounds(v.constantVariable)
		if err != nil {
			return nil, nil, err
		}
		v.lowerInf, v.upperInf = lo == nil, hi == nil
		v.lowerOpen, v.upperOpen = lo == nil || lo.open, hi == nil || hi.open
		if lo != nil {
			lower = &lo.value
		}
		if hi != nil {
			upper = &hi.value
		}
	}
	return lower, upper, nil
}

func (v *RationalInterval) mutable() error {
	_, _, err := v.resolved()
	if err == nil && v.constantNode != nil {
		return arena.ErrInvalidSlot
	}
	return err
}

func (v *RationalInterval) SetConstant(node *RationalNode, variable uint32) error {
	if _, _, err := v.resolved(); err != nil {
		return err
	}
	if node == nil || node.numbers != v.numbers {
		return arena.ErrInvalidSlot
	}
	if _, _, err := node.Bounds(variable); err != nil {
		return err
	}
	if v.constantNode == nil {
		v.mutableSnapshot = true
		v.savedLowerInf, v.savedUpperInf = v.lowerInf, v.upperInf
		v.savedLowerOpen, v.savedUpperOpen = v.lowerOpen, v.upperOpen
	}
	v.constantNode, v.constantVariable = node, variable
	_, _, err := v.resolved()
	return err
}

func (v *RationalInterval) SetMutable() error {
	if _, _, err := v.resolved(); err != nil {
		return err
	}
	if v.constantNode == nil {
		return nil
	}
	v.constantNode = nil
	if v.mutableSnapshot {
		v.lowerInf, v.upperInf = v.savedLowerInf, v.savedUpperInf
		v.lowerOpen, v.upperOpen = v.savedLowerOpen, v.savedUpperOpen
	}
	v.mutableSnapshot = false
	return nil
}

func (v *RationalInterval) lowerKind() interval.ExtendedKind {
	if v.lowerInf {
		return interval.MinusInfinity
	}
	return interval.Finite
}

func (v *RationalInterval) upperKind() interval.ExtendedKind {
	if v.upperInf {
		return interval.PlusInfinity
	}
	return interval.Finite
}

func (v *RationalInterval) pairView(other *RationalInterval) (*numeral.Rational, *numeral.Rational, error) {
	if err := v.mutable(); err != nil {
		return nil, nil, err
	}
	lo, hi, err := other.resolved()
	if err != nil {
		return nil, nil, err
	}
	if v.numbers != other.numbers {
		return nil, nil, arena.ErrInvalidSlot
	}
	return lo, hi, nil
}

func (v *RationalInterval) SetFinite(lower, upper *numeral.Rational, lowerOpen, upperOpen bool) error {
	if err := v.mutable(); err != nil {
		return err
	}
	e := interval.RationalExtended{Numbers: v.numbers}
	equal, err := e.Equal(lower, interval.Finite, upper, interval.Finite)
	if err != nil {
		return err
	}
	if equal {
		if lowerOpen || upperOpen {
			return interval.ErrExtendedDomain
		}
	} else {
		less, err := e.Less(lower, interval.Finite, upper, interval.Finite)
		if err != nil {
			return err
		}
		if !less {
			return interval.ErrExtendedDomain
		}
	}
	if err := v.numbers.Copy(&v.lower, lower); err != nil {
		return err
	}
	if err := v.numbers.Copy(&v.upper, upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = false, false
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return nil
}

func (v *RationalInterval) ResetLower() error {
	if err := v.mutable(); err != nil {
		return err
	}
	if err := v.numbers.Reset(&v.lower); err != nil {
		return err
	}
	v.lowerInf, v.lowerOpen = true, true
	return nil
}

func (v *RationalInterval) ResetUpper() error {
	if err := v.mutable(); err != nil {
		return err
	}
	if err := v.numbers.Reset(&v.upper); err != nil {
		return err
	}
	v.upperInf, v.upperOpen = true, true
	return nil
}

func (v *RationalInterval) Reset() error {
	if err := v.ResetLower(); err != nil {
		return err
	}
	return v.ResetUpper()
}

func (v *RationalInterval) SetFrom(src *RationalInterval) error {
	if v == src {
		return v.open()
	}
	if err := v.mutable(); err != nil {
		return err
	}
	slo, shi, err := src.resolved()
	if err != nil {
		return err
	}
	if v.numbers != src.numbers {
		return arena.ErrInvalidSlot
	}
	if !src.lowerInf {
		if err := v.numbers.Copy(&v.lower, slo); err != nil {
			return err
		}
	}
	if !src.upperInf {
		if err := v.numbers.Copy(&v.upper, shi); err != nil {
			return err
		}
	}
	v.lowerInf, v.upperInf = src.lowerInf, src.upperInf
	v.lowerOpen, v.upperOpen = src.lowerOpen, src.upperOpen
	return v.CheckInvariant()
}

func (v *RationalInterval) CheckInvariant() error {
	lo, hi, err := v.resolved()
	if err != nil {
		return err
	}
	e := interval.RationalExtended{Numbers: v.numbers}
	equal, err := e.Equal(lo, v.lowerKind(), hi, v.upperKind())
	if err != nil {
		return err
	}
	if equal {
		if v.lowerOpen || v.upperOpen {
			return interval.ErrExtendedDomain
		}
		return nil
	}
	less, err := e.Less(lo, v.lowerKind(), hi, v.upperKind())
	if err != nil {
		return err
	}
	if !less {
		return interval.ErrExtendedDomain
	}
	return nil
}

func (v *RationalInterval) ContainsZero() (bool, error) {
	lo, hi, err := v.resolved()
	if err != nil {
		return false, err
	}
	e := interval.RationalExtended{Numbers: v.numbers}
	neg, err := e.IsNeg(lo, v.lowerKind())
	if err != nil {
		return false, err
	}
	zero, err := e.IsZero(lo, v.lowerKind())
	if err != nil {
		return false, err
	}
	if !neg && (!zero || v.lowerOpen) {
		return false, nil
	}
	pos, err := e.IsPos(hi, v.upperKind())
	if err != nil {
		return false, err
	}
	zero, err = e.IsZero(hi, v.upperKind())
	if err != nil {
		return false, err
	}
	return pos || zero && !v.upperOpen, nil
}

func (v *RationalInterval) Display(w io.Writer) error {
	lo, hi, err := v.resolved()
	if err != nil {
		return err
	}
	left, right := "[", "]"
	if v.lowerOpen {
		left = "("
	}
	if v.upperOpen {
		right = ")"
	}
	if err := writeSubpavingIntervalText(w, left); err != nil {
		return err
	}
	e := interval.RationalExtended{Numbers: v.numbers}
	if err := e.Display(w, lo, v.lowerKind()); err != nil {
		return err
	}
	if err := writeSubpavingIntervalText(w, ", "); err != nil {
		return err
	}
	if err := e.Display(w, hi, v.upperKind()); err != nil {
		return err
	}
	return writeSubpavingIntervalText(w, right)
}

func (v *RationalInterval) Close() error {
	if v == nil || v.self != v {
		return arena.ErrInvalidSlot
	}
	if v.closed {
		return nil
	}
	if err := v.numbers.Del(&v.lower); err != nil {
		return err
	}
	if err := v.numbers.Del(&v.upper); err != nil {
		return err
	}
	v.closed = true
	return nil
}
