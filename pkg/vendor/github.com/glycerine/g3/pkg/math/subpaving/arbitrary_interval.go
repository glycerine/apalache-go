package subpaving

// Source: src/math/subpaving/subpaving_t.h:239-316 and
// src/math/interval/interval_def.h:597-680, concrete config_mpf instance.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// ArbitraryInterval owns its two mutable MPF endpoint numerals. The borrowed
// wrapper and its core manager must outlive the interval.
type ArbitraryInterval struct {
	self                 *ArbitraryInterval
	numbers              *numeral.ArbitraryFloatNumeralManager
	lower, upper         numeral.ArbitraryFloat
	constantNode         *ArbitraryNode
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

func NewArbitraryInterval(m *numeral.ArbitraryFloatNumeralManager) (*ArbitraryInterval, error) {
	if m == nil {
		return nil, arena.ErrInvalidSlot
	}
	v := &ArbitraryInterval{numbers: m, lowerInf: true, upperInf: true, lowerOpen: true, upperOpen: true}
	v.self = v
	return v, nil
}

func (v *ArbitraryInterval) endpoints() (*numeral.ArbitraryFloat, *numeral.ArbitraryFloat, error) {
	if v == nil || v.self != v || v.numbers == nil {
		return nil, nil, arena.ErrInvalidSlot
	}
	if v.closed {
		return nil, nil, arena.ErrClosed
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

func (v *ArbitraryInterval) mutable() error {
	_, _, err := v.endpoints()
	if err == nil && v.constantNode != nil {
		return arena.ErrInvalidSlot
	}
	return err
}

// SetConstant borrows the live root's current bounds. Later reads see bounds
// published after binding, as in interval::set_constant.
func (v *ArbitraryInterval) SetConstant(node *ArbitraryNode, variable uint32) error {
	if _, _, err := v.endpoints(); err != nil {
		return err
	}
	if node == nil || node.context == nil || node.context.numbers != v.numbers {
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
	_, _, err := v.endpoints()
	return err
}

// SetMutable detaches the borrowed node view and restores the interval's
// owned endpoint slots for source scratch reuse.
func (v *ArbitraryInterval) SetMutable() error {
	if _, _, err := v.endpoints(); err != nil {
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

func (v *ArbitraryInterval) lowerKind() interval.ExtendedKind {
	if v.lowerInf {
		return interval.MinusInfinity
	}
	return interval.Finite
}

func (v *ArbitraryInterval) upperKind() interval.ExtendedKind {
	if v.upperInf {
		return interval.PlusInfinity
	}
	return interval.Finite
}

func (v *ArbitraryInterval) SetFinite(lower, upper *numeral.ArbitraryFloat, lowerOpen, upperOpen bool) error {
	if err := v.mutable(); err != nil {
		return err
	}
	lo, hi, err := v.endpoints()
	if err != nil {
		return err
	}
	e := interval.ArbitraryExtended{Numbers: v.numbers}
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
	if err := v.numbers.Copy(lo, lower); err != nil {
		return err
	}
	if err := v.numbers.Copy(hi, upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = false, false
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return nil
}

func (v *ArbitraryInterval) ResetLower() error {
	if err := v.mutable(); err != nil {
		return err
	}
	lo, _, err := v.endpoints()
	if err != nil {
		return err
	}
	if err := v.numbers.Reset(lo); err != nil {
		return err
	}
	v.lowerInf, v.lowerOpen = true, true
	return nil
}

func (v *ArbitraryInterval) ResetUpper() error {
	if err := v.mutable(); err != nil {
		return err
	}
	_, hi, err := v.endpoints()
	if err != nil {
		return err
	}
	if err := v.numbers.Reset(hi); err != nil {
		return err
	}
	v.upperInf, v.upperOpen = true, true
	return nil
}

func (v *ArbitraryInterval) Reset() error {
	if err := v.ResetLower(); err != nil {
		return err
	}
	return v.ResetUpper()
}

func (v *ArbitraryInterval) SetFrom(src *ArbitraryInterval) error {
	if v == src {
		_, _, err := v.endpoints()
		return err
	}
	if err := v.mutable(); err != nil {
		return err
	}
	lo, hi, err := v.endpoints()
	if err != nil {
		return err
	}
	slo, shi, err := src.endpoints()
	if err != nil {
		return err
	}
	if v.numbers != src.numbers {
		return arena.ErrInvalidSlot
	}
	if !src.lowerInf {
		if err := v.numbers.Copy(lo, slo); err != nil {
			return err
		}
	}
	if !src.upperInf {
		if err := v.numbers.Copy(hi, shi); err != nil {
			return err
		}
	}
	v.lowerInf, v.upperInf = src.lowerInf, src.upperInf
	v.lowerOpen, v.upperOpen = src.lowerOpen, src.upperOpen
	return v.CheckInvariant()
}

func (v *ArbitraryInterval) CheckInvariant() error {
	lo, hi, err := v.endpoints()
	if err != nil {
		return err
	}
	e := interval.ArbitraryExtended{Numbers: v.numbers}
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

func (v *ArbitraryInterval) ContainsZero() (bool, error) {
	lo, hi, err := v.endpoints()
	if err != nil {
		return false, err
	}
	e := interval.ArbitraryExtended{Numbers: v.numbers}
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

func (v *ArbitraryInterval) Display(w io.Writer) error {
	lo, hi, err := v.endpoints()
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
	e := interval.ArbitraryExtended{Numbers: v.numbers}
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

func (v *ArbitraryInterval) Close() error {
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
