package subpaving

// Source: src/math/subpaving/subpaving_t.h:239-316 and selected
// src/math/interval/interval_def.h:597-837, concrete config_mpfx instance.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// FixedInterval is the mutable, numeral-owning subpaving scratch interval.
// Constant node/variable views borrow bounds and are represented separately.
type FixedInterval struct {
	self                   *FixedInterval
	numbers                *numeral.FixedPointManager
	lower, upper           *numeral.FixedPoint
	ownedLower, ownedUpper *numeral.FixedPoint
	constantNode           *FixedNode
	constantVariable       uint32
	lowerInf, upperInf     bool
	lowerOpen, upperOpen   bool
	mutableSnapshot        bool
	savedLowerInf          bool
	savedUpperInf          bool
	savedLowerOpen         bool
	savedUpperOpen         bool
	closed                 bool
}

func NewFixedInterval(m *numeral.FixedPointManager) (*FixedInterval, error) {
	if m == nil {
		return nil, arena.ErrInvalidSlot
	}
	lower, err := m.New()
	if err != nil {
		return nil, err
	}
	upper, err := m.New()
	if err != nil {
		return nil, errors.Join(err, lower.Close())
	}
	v := &FixedInterval{numbers: m, lower: lower, upper: upper, ownedLower: lower, ownedUpper: upper, lowerInf: true, upperInf: true, lowerOpen: true, upperOpen: true}
	v.self = v
	return v, nil
}

func (v *FixedInterval) open() error {
	if v == nil || v.self != v || v.numbers == nil || v.lower == nil || v.upper == nil {
		return arena.ErrInvalidSlot
	}
	if v.closed {
		return arena.ErrClosed
	}
	if v.constantNode != nil {
		lower, err := v.constantNode.bound(v.constantVariable, true)
		if err != nil {
			return err
		}
		upper, err := v.constantNode.bound(v.constantVariable, false)
		if err != nil {
			return err
		}
		v.lower, v.upper = v.ownedLower, v.ownedUpper
		v.lowerInf, v.upperInf = lower == nil, upper == nil
		v.lowerOpen, v.upperOpen = lower == nil || lower.open, upper == nil || upper.open
		if lower != nil {
			v.lower = lower.value
		}
		if upper != nil {
			v.upper = upper.value
		}
	}
	return nil
}

// SetConstant binds this scratch interval to a live node/variable pair. Each
// subsequent read resolves the node's current bounds, as the source does.
func (v *FixedInterval) SetConstant(node *FixedNode, variable uint32) error {
	if err := v.open(); err != nil {
		return err
	}
	if node == nil || node.numbers != v.numbers {
		return arena.ErrInvalidSlot
	}
	if _, err := node.bound(variable, true); err != nil {
		return err
	}
	if v.constantNode == nil {
		v.mutableSnapshot = true
		v.savedLowerInf, v.savedUpperInf = v.lowerInf, v.upperInf
		v.savedLowerOpen, v.savedUpperOpen = v.lowerOpen, v.upperOpen
	}
	v.constantNode, v.constantVariable = node, variable
	return v.open()
}

// SetMutable returns to the scratch endpoints and flags hidden by a live
// node view, matching the source interval's flag-only set_mutable operation.
func (v *FixedInterval) SetMutable() error {
	if err := v.open(); err != nil {
		return err
	}
	if v.constantNode == nil {
		return nil
	}
	v.constantNode = nil
	v.lower, v.upper = v.ownedLower, v.ownedUpper
	if v.mutableSnapshot {
		v.lowerInf, v.upperInf = v.savedLowerInf, v.savedUpperInf
		v.lowerOpen, v.upperOpen = v.savedLowerOpen, v.savedUpperOpen
	}
	v.mutableSnapshot = false
	return nil
}

func (v *FixedInterval) mutable() error {
	if err := v.open(); err != nil {
		return err
	}
	if v.constantNode != nil {
		return arena.ErrInvalidSlot
	}
	return nil
}

func (v *FixedInterval) pair(other *FixedInterval) error {
	if err := v.mutable(); err != nil {
		return err
	}
	if err := other.open(); err != nil {
		return err
	}
	if v.numbers != other.numbers {
		return arena.ErrInvalidSlot
	}
	return nil
}

func (v *FixedInterval) lowerKind() interval.ExtendedKind {
	if v.lowerInf {
		return interval.MinusInfinity
	}
	return interval.Finite
}

func (v *FixedInterval) upperKind() interval.ExtendedKind {
	if v.upperInf {
		return interval.PlusInfinity
	}
	return interval.Finite
}

func (v *FixedInterval) SetFinite(lower, upper *numeral.FixedPoint, lowerOpen, upperOpen bool) error {
	if err := v.mutable(); err != nil {
		return err
	}
	e := interval.FixedExtended{Numbers: v.numbers}
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
	if err := v.numbers.Copy(v.lower, lower); err != nil {
		return err
	}
	if err := v.numbers.Copy(v.upper, upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = false, false
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return nil
}

func (v *FixedInterval) ResetLower() error {
	if err := v.mutable(); err != nil {
		return err
	}
	if err := v.numbers.Reset(v.lower); err != nil {
		return err
	}
	v.lowerInf, v.lowerOpen = true, true
	return nil
}

func (v *FixedInterval) ResetUpper() error {
	if err := v.mutable(); err != nil {
		return err
	}
	if err := v.numbers.Reset(v.upper); err != nil {
		return err
	}
	v.upperInf, v.upperOpen = true, true
	return nil
}

func (v *FixedInterval) Reset() error {
	if err := v.ResetLower(); err != nil {
		return err
	}
	return v.ResetUpper()
}

func (v *FixedInterval) SetFrom(src *FixedInterval) error {
	if v == src {
		return v.open()
	}
	if err := v.pair(src); err != nil {
		return err
	}
	if !src.lowerInf {
		if err := v.numbers.Copy(v.lower, src.lower); err != nil {
			return err
		}
	}
	if !src.upperInf {
		if err := v.numbers.Copy(v.upper, src.upper); err != nil {
			return err
		}
	}
	v.lowerInf, v.upperInf = src.lowerInf, src.upperInf
	v.lowerOpen, v.upperOpen = src.lowerOpen, src.upperOpen
	return v.CheckInvariant()
}

func (v *FixedInterval) CheckInvariant() error {
	if err := v.open(); err != nil {
		return err
	}
	e := interval.FixedExtended{Numbers: v.numbers}
	equal, err := e.Equal(v.lower, v.lowerKind(), v.upper, v.upperKind())
	if err != nil {
		return err
	}
	if equal {
		if v.lowerOpen || v.upperOpen {
			return interval.ErrExtendedDomain
		}
		return nil
	}
	less, err := e.Less(v.lower, v.lowerKind(), v.upper, v.upperKind())
	if err != nil {
		return err
	}
	if !less {
		return interval.ErrExtendedDomain
	}
	return nil
}

func (v *FixedInterval) ContainsZero() (bool, error) {
	if err := v.open(); err != nil {
		return false, err
	}
	e := interval.FixedExtended{Numbers: v.numbers}
	neg, err := e.IsNeg(v.lower, v.lowerKind())
	if err != nil {
		return false, err
	}
	zero, err := e.IsZero(v.lower, v.lowerKind())
	if err != nil {
		return false, err
	}
	if !neg && (!zero || v.lowerOpen) {
		return false, nil
	}
	pos, err := e.IsPos(v.upper, v.upperKind())
	if err != nil {
		return false, err
	}
	zero, err = e.IsZero(v.upper, v.upperKind())
	if err != nil {
		return false, err
	}
	return pos || zero && !v.upperOpen, nil
}

func (v *FixedInterval) LowerIsNeg() (bool, error) {
	if err := v.open(); err != nil {
		return false, err
	}
	return (interval.FixedExtended{Numbers: v.numbers}).IsNeg(v.lower, v.lowerKind())
}

func (v *FixedInterval) Display(w io.Writer) error {
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
	if err := writeSubpavingIntervalText(w, left); err != nil {
		return err
	}
	e := interval.FixedExtended{Numbers: v.numbers}
	if err := e.Display(w, v.lower, v.lowerKind()); err != nil {
		return err
	}
	if err := writeSubpavingIntervalText(w, ", "); err != nil {
		return err
	}
	if err := e.Display(w, v.upper, v.upperKind()); err != nil {
		return err
	}
	return writeSubpavingIntervalText(w, right)
}

func (v *FixedInterval) Close() error {
	if v == nil || v.self != v {
		return arena.ErrInvalidSlot
	}
	if v.closed {
		return nil
	}
	if err := errors.Join(v.ownedLower.Close(), v.ownedUpper.Close()); err != nil {
		return err
	}
	v.closed = true
	return nil
}
