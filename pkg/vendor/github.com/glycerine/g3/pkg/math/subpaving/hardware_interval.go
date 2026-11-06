package subpaving

// Source: src/math/subpaving/subpaving_t.h:239-316 and
// src/math/interval/interval_def.h:597-709, concrete config_hwf instance.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// HardwareInterval owns two scalar binary64 endpoint slots. Its numeral
// manager is borrowed; the interval must be closed before that owner ends.
type HardwareInterval struct {
	self                 *HardwareInterval
	numbers              *numeral.HardwareFloatNumeralManager
	lower, upper         numeral.HardwareFloat
	constantNode         *HardwareNode
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

func NewHardwareInterval(m *numeral.HardwareFloatNumeralManager) (*HardwareInterval, error) {
	if m == nil {
		return nil, arena.ErrInvalidSlot
	}
	v := &HardwareInterval{numbers: m, lowerInf: true, upperInf: true, lowerOpen: true, upperOpen: true}
	v.self = v
	return v, nil
}
func (v *HardwareInterval) endpoints() (*numeral.HardwareFloat, *numeral.HardwareFloat, error) {
	if v == nil || v.self != v || v.numbers == nil {
		return nil, nil, arena.ErrInvalidSlot
	}
	if v.closed {
		return nil, nil, arena.ErrClosed
	}
	lower, upper := &v.lower, &v.upper
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
			lower = &lo.value
		}
		if hi != nil {
			upper = &hi.value
		}
	}
	return lower, upper, nil
}

// SetConstant borrows a live node's bound views. Later reads observe bounds
// pushed after this binding, as in the source interval::set_constant.
func (v *HardwareInterval) SetConstant(node *HardwareNode, variable uint32) error {
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

// SetMutable restores the scratch interval state hidden by a constant view,
// matching interval::set_mutable's flag-only transition.
func (v *HardwareInterval) SetMutable() error {
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

func (v *HardwareInterval) mutable() error {
	if _, _, err := v.endpoints(); err != nil {
		return err
	}
	if v.constantNode != nil {
		return arena.ErrInvalidSlot
	}
	return nil
}

func (v *HardwareInterval) writablePair(other *HardwareInterval) error {
	if err := v.mutable(); err != nil {
		return err
	}
	return v.pair(other)
}
func (v *HardwareInterval) lowerKind() interval.ExtendedKind {
	if v.lowerInf {
		return interval.MinusInfinity
	}
	return interval.Finite
}
func (v *HardwareInterval) upperKind() interval.ExtendedKind {
	if v.upperInf {
		return interval.PlusInfinity
	}
	return interval.Finite
}
func (v *HardwareInterval) pair(other *HardwareInterval) error {
	if _, _, err := v.endpoints(); err != nil {
		return err
	}
	if _, _, err := other.endpoints(); err != nil {
		return err
	}
	if v.numbers != other.numbers {
		return arena.ErrInvalidSlot
	}
	return nil
}
func (v *HardwareInterval) SetFinite(lower, upper *numeral.HardwareFloat, lowerOpen, upperOpen bool) error {
	if err := v.mutable(); err != nil {
		return err
	}
	lo, hi, err := v.endpoints()
	if err != nil {
		return err
	}
	e := interval.HardwareExtended{Numbers: v.numbers}
	equal := e.Equal(lower, interval.Finite, upper, interval.Finite)
	if equal {
		if lowerOpen || upperOpen {
			return interval.ErrExtendedDomain
		}
	} else if !e.Less(lower, interval.Finite, upper, interval.Finite) {
		return interval.ErrExtendedDomain
	}
	if err = v.numbers.Set(lo, lower); err != nil {
		return err
	}
	if err = v.numbers.Set(hi, upper); err != nil {
		return err
	}
	v.lowerInf, v.upperInf = false, false
	v.lowerOpen, v.upperOpen = lowerOpen, upperOpen
	return nil
}
func (v *HardwareInterval) ResetLower() error {
	if err := v.mutable(); err != nil {
		return err
	}
	lo, _, err := v.endpoints()
	if err != nil {
		return err
	}
	if err = v.numbers.Reset(lo); err != nil {
		return err
	}
	v.lowerInf, v.lowerOpen = true, true
	return nil
}
func (v *HardwareInterval) ResetUpper() error {
	if err := v.mutable(); err != nil {
		return err
	}
	_, hi, err := v.endpoints()
	if err != nil {
		return err
	}
	if err = v.numbers.Reset(hi); err != nil {
		return err
	}
	v.upperInf, v.upperOpen = true, true
	return nil
}
func (v *HardwareInterval) Reset() error {
	if err := v.ResetLower(); err != nil {
		return err
	}
	return v.ResetUpper()
}
func (v *HardwareInterval) SetFrom(src *HardwareInterval) error {
	if v == src {
		_, _, err := v.endpoints()
		return err
	}
	if err := v.writablePair(src); err != nil {
		return err
	}
	lo, hi, _ := v.endpoints()
	slo, shi, _ := src.endpoints()
	if !src.lowerInf {
		if err := v.numbers.Set(lo, slo); err != nil {
			return err
		}
	}
	if !src.upperInf {
		if err := v.numbers.Set(hi, shi); err != nil {
			return err
		}
	}
	v.lowerInf, v.upperInf = src.lowerInf, src.upperInf
	v.lowerOpen, v.upperOpen = src.lowerOpen, src.upperOpen
	return v.CheckInvariant()
}
func (v *HardwareInterval) CheckInvariant() error {
	lo, hi, err := v.endpoints()
	if err != nil {
		return err
	}
	e := interval.HardwareExtended{Numbers: v.numbers}
	if e.Equal(lo, v.lowerKind(), hi, v.upperKind()) {
		if v.lowerOpen || v.upperOpen {
			return interval.ErrExtendedDomain
		}
		return nil
	}
	if !e.Less(lo, v.lowerKind(), hi, v.upperKind()) {
		return interval.ErrExtendedDomain
	}
	return nil
}
func (v *HardwareInterval) ContainsZero() (bool, error) {
	lo, hi, err := v.endpoints()
	if err != nil {
		return false, err
	}
	e := interval.HardwareExtended{Numbers: v.numbers}
	if !e.IsNeg(lo, v.lowerKind()) && (!e.IsZero(lo, v.lowerKind()) || v.lowerOpen) {
		return false, nil
	}
	return e.IsPos(hi, v.upperKind()) || e.IsZero(hi, v.upperKind()) && !v.upperOpen, nil
}
func (v *HardwareInterval) Contains(value *numeral.HardwareFloat) (bool, error) {
	lo, hi, err := v.endpoints()
	if err != nil {
		return false, err
	}
	m := v.numbers
	if !v.lowerInf && (m.Less(value, lo) || m.Equal(value, lo) && v.lowerOpen) {
		return false, nil
	}
	if !v.upperInf && (m.Greater(value, hi) || m.Equal(value, hi) && v.upperOpen) {
		return false, nil
	}
	return true, nil
}
func (v *HardwareInterval) Equal(other *HardwareInterval) (bool, error) {
	if err := v.pair(other); err != nil {
		return false, err
	}
	lo, hi, _ := v.endpoints()
	olo, ohi, _ := other.endpoints()
	e := interval.HardwareExtended{Numbers: v.numbers}
	return e.Equal(lo, v.lowerKind(), olo, other.lowerKind()) && e.Equal(hi, v.upperKind(), ohi, other.upperKind()) && v.lowerOpen == other.lowerOpen && v.upperOpen == other.upperOpen, nil
}
func (v *HardwareInterval) Before(other *HardwareInterval) (bool, error) {
	if err := v.pair(other); err != nil {
		return false, err
	}
	_, hi, _ := v.endpoints()
	lo, _, _ := other.endpoints()
	if v.upperInf || other.lowerInf {
		return false, nil
	}
	return v.numbers.Less(hi, lo) || v.upperOpen && v.numbers.Equal(hi, lo), nil
}
func (v *HardwareInterval) Display(w io.Writer) error {
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
	if err = writeSubpavingIntervalText(w, left); err != nil {
		return err
	}
	e := interval.HardwareExtended{Numbers: v.numbers}
	if err = e.Display(w, lo, v.lowerKind()); err != nil {
		return err
	}
	if err = writeSubpavingIntervalText(w, ", "); err != nil {
		return err
	}
	if err = e.Display(w, hi, v.upperKind()); err != nil {
		return err
	}
	return writeSubpavingIntervalText(w, right)
}
func (v *HardwareInterval) Close() error {
	if v == nil || v.self != v {
		return arena.ErrInvalidSlot
	}
	v.closed = true
	return nil
}
