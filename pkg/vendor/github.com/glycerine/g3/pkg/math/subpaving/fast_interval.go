package subpaving

// Source: src/math/subpaving/subpaving_t.h:239-316 and
// src/math/interval/interval_def.h:597-680, concrete config_mpff instance.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// FastInterval owns mutable fast-float endpoints. A constant view borrows
// bounds from a live node and resolves their current values on each read.
type FastInterval struct {
	self                 *FastInterval
	numbers              *numeral.FastFloatManager
	lower, upper         *numeral.ScopedFastFloat
	constantNode         *FastNode
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

func NewFastInterval(m *numeral.FastFloatManager) (*FastInterval, error) {
	if m == nil {
		return nil, arena.ErrInvalidSlot
	}
	lower, err := numeral.NewScopedFastFloat(m)
	if err != nil {
		return nil, err
	}
	upper, err := numeral.NewScopedFastFloat(m)
	if err != nil {
		return nil, errors.Join(err, lower.Close())
	}
	v := &FastInterval{numbers: m, lower: lower, upper: upper, lowerInf: true, upperInf: true, lowerOpen: true, upperOpen: true}
	v.self = v
	return v, nil
}

func (v *FastInterval) endpoints() (*numeral.FastFloat, *numeral.FastFloat, error) {
	if v == nil || v.self != v || v.numbers == nil || v.lower == nil || v.upper == nil {
		return nil, nil, arena.ErrInvalidSlot
	}
	if v.closed {
		return nil, nil, arena.ErrClosed
	}
	lower, err := v.lower.Get()
	if err != nil {
		return nil, nil, err
	}
	upper, err := v.upper.Get()
	if err != nil {
		return nil, nil, err
	}
	if v.constantNode != nil {
		lo, err := v.constantNode.bound(v.constantVariable, true)
		if err != nil {
			return nil, nil, err
		}
		hi, err := v.constantNode.bound(v.constantVariable, false)
		if err != nil {
			return nil, nil, err
		}
		v.lowerInf, v.upperInf = lo == nil, hi == nil
		v.lowerOpen, v.upperOpen = lo == nil || lo.open, hi == nil || hi.open
		if lo != nil {
			lower = lo.value
		}
		if hi != nil {
			upper = hi.value
		}
	}
	return lower, upper, nil
}

// SetConstant binds this interval to a node/variable pair. Subsequent reads
// observe bounds pushed on that node after the binding.
func (v *FastInterval) SetConstant(node *FastNode, variable uint32) error {
	if _, _, err := v.endpoints(); err != nil {
		return err
	}
	if node == nil || node.numbers != v.numbers {
		return arena.ErrInvalidSlot
	}
	if _, err := node.bound(variable, true); err != nil {
		return err
	}
	if _, err := node.bound(variable, false); err != nil {
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

// SetMutable detaches the live node view. The source only clears its constant
// flag, so restore the interval's owned scratch flags as they were before the
// view was bound.
func (v *FastInterval) SetMutable() error {
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

func (v *FastInterval) mutable() error {
	if _, _, err := v.endpoints(); err != nil {
		return err
	}
	if v.constantNode != nil {
		return arena.ErrInvalidSlot
	}
	return nil
}

func (v *FastInterval) lowerKind() interval.ExtendedKind {
	if v.lowerInf {
		return interval.MinusInfinity
	}
	return interval.Finite
}

func (v *FastInterval) upperKind() interval.ExtendedKind {
	if v.upperInf {
		return interval.PlusInfinity
	}
	return interval.Finite
}

func (v *FastInterval) SetFinite(lower, upper *numeral.FastFloat, lowerOpen, upperOpen bool) error {
	if err := v.mutable(); err != nil {
		return err
	}
	lo, hi, err := v.endpoints()
	if err != nil {
		return err
	}
	e := interval.FastExtended{Numbers: v.numbers}
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

func (v *FastInterval) ResetLower() error {
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

func (v *FastInterval) ResetUpper() error {
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

func (v *FastInterval) Reset() error {
	if err := v.ResetLower(); err != nil {
		return err
	}
	return v.ResetUpper()
}

func (v *FastInterval) SetFrom(src *FastInterval) error {
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

func (v *FastInterval) CheckInvariant() error {
	lo, hi, err := v.endpoints()
	if err != nil {
		return err
	}
	e := interval.FastExtended{Numbers: v.numbers}
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

func (v *FastInterval) ContainsZero() (bool, error) {
	lo, hi, err := v.endpoints()
	if err != nil {
		return false, err
	}
	e := interval.FastExtended{Numbers: v.numbers}
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

func (v *FastInterval) Display(w io.Writer) error {
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
	e := interval.FastExtended{Numbers: v.numbers}
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

func writeSubpavingIntervalText(w io.Writer, s string) error {
	n, err := io.WriteString(w, s)
	if err != nil {
		return err
	}
	if n != len(s) {
		return io.ErrShortWrite
	}
	return nil
}

func (v *FastInterval) Close() error {
	if v == nil || v.self != v {
		return arena.ErrInvalidSlot
	}
	if v.closed {
		return nil
	}
	if err := errors.Join(v.lower.Close(), v.upper.Close()); err != nil {
		return err
	}
	v.closed = true
	return nil
}
