package lp

// Source: src/util/lp/stacked_vector.h, numeric_pair<mpq> LAR bounds.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

var ErrPairVectorPending = errors.New("lp: scoped pair vector has an unfinished copied value")

type stackedPairChange struct {
	index uint32
	old   RationalPair
}

// StackedPairVector owns exact values and replacement snapshots in a borrowed
// manager. Construct with NewStackedPairVector and Close before closing that
// manager. Never shallow-copy. At and Values borrow read-only numerals; replace
// through Set. Any mutation error leaves all completed and partial roots owned
// here; Close releases them and may itself be retried after a cleanup error.
type StackedPairVector struct {
	manager                  *numeral.RationalManager
	values                   []RationalPair
	changes                  []stackedPairChange
	vectorSizes, changeSizes []uint32
	// Actual unfinished Append/Set copy, not an allocation simulation.
	pending    RationalPair
	hasPending bool
	copyRoots  numeral.NumericAllocationRoots
}

func NewStackedPairVector(m *numeral.RationalManager) *StackedPairVector {
	return &StackedPairVector{manager: m}
}
func (v *StackedPairVector) Size() uint32           { return uint32(len(v.values)) }
func (v *StackedPairVector) Values() []RationalPair { return v.values }
func (v *StackedPairVector) At(i uint32) (*RationalPair, error) {
	if uint64(i) >= uint64(len(v.values)) {
		return nil, ErrBounds
	}
	return &v.values[i], nil
}
func (v *StackedPairVector) Append(value *RationalPair) error {
	if v.unfinished() {
		return ErrPairVectorPending
	}
	v.pending = NewRationalPair()
	v.hasPending = true
	if err := v.copyPair(&v.pending, value); err != nil {
		return err
	}
	v.values = append(v.values, v.pending) // move owning tokens into native backing
	v.pending = RationalPair{}
	v.hasPending = false
	return nil
}
func (v *StackedPairVector) IncreaseSizeByOne() error {
	if v.unfinished() {
		return ErrPairVectorPending
	}
	v.values = append(v.values, NewRationalPair())
	return nil
}
func (v *StackedPairVector) Set(i uint32, value *RationalPair) error {
	if v.unfinished() {
		return ErrPairVectorPending
	}
	if uint64(i) >= uint64(len(v.values)) {
		return ErrBounds
	}
	equal, err := v.values[i].Equal(v.manager, value)
	if err != nil || equal {
		return err
	}
	v.pending = NewRationalPair()
	v.hasPending = true
	if err := v.copyPair(&v.pending, &v.values[i]); err != nil {
		return err
	}
	v.changes = append(v.changes, stackedPairChange{index: i, old: v.pending})
	v.pending = RationalPair{}
	v.hasPending = false
	return v.copyPair(&v.values[i], value)
}
func (v *StackedPairVector) Push() error {
	if v.unfinished() {
		return ErrPairVectorPending
	}
	v.changeSizes = append(v.changeSizes, uint32(len(v.changes)))
	v.vectorSizes = append(v.vectorSizes, uint32(len(v.values)))
	return nil
}
func (v *StackedPairVector) PeekSize(k uint32) (uint32, error) {
	if k == 0 || uint64(k) > uint64(len(v.vectorSizes)) {
		return 0, ErrScope
	}
	return v.vectorSizes[len(v.vectorSizes)-int(k)], nil
}
func (v *StackedPairVector) Pop(k uint32) error {
	if v.unfinished() {
		return ErrPairVectorPending
	}
	if k == 0 || uint64(k) > uint64(len(v.vectorSizes)) {
		return ErrScope
	}
	firstScope := len(v.vectorSizes) - int(k)
	size := v.vectorSizes[firstScope]
	// Source vector resize destroys the removed tail before popping scope sizes.
	for uint64(len(v.values)) > uint64(size) {
		last := len(v.values) - 1
		if err := v.values[last].Del(v.manager); err != nil {
			return err
		}
		v.values = v.values[:last]
	}
	v.vectorSizes = v.vectorSizes[:firstScope]
	firstChange := v.changeSizes[firstScope]
	v.changeSizes = v.changeSizes[:firstScope]
	for j := len(v.changes); j > int(firstChange); {
		j--
		change := &v.changes[j]
		if uint64(change.index) < uint64(len(v.values)) {
			if err := v.copyPair(&v.values[change.index], &change.old); err != nil {
				return err
			}
		}
	}
	// As upstream, restoring copies precede destruction of the journal tail.
	for len(v.changes) > int(firstChange) {
		last := len(v.changes) - 1
		if err := v.changes[last].old.Del(v.manager); err != nil {
			return err
		}
		v.changes = v.changes[:last]
	}
	return nil
}
func (v *StackedPairVector) Close() error {
	if v.hasPending {
		if err := v.pending.Del(v.manager); err != nil {
			return err
		}
		v.hasPending = false
		v.pending = RationalPair{}
	}
	for len(v.values) > 0 {
		last := len(v.values) - 1
		if err := v.values[last].Del(v.manager); err != nil {
			return err
		}
		v.values = v.values[:last]
	}
	for len(v.changes) > 0 {
		last := len(v.changes) - 1
		if err := v.changes[last].old.Del(v.manager); err != nil {
			return err
		}
		v.changes = v.changes[:last]
	}
	if err := numeral.CleanupNumericAllocationRoots(v.manager.Z, &v.copyRoots); err != nil {
		return err
	}
	v.values = nil
	v.changes = nil
	v.vectorSizes = nil
	v.changeSizes = nil
	return nil
}

func (v *StackedPairVector) unfinished() bool {
	return v.hasPending || numeral.NumericAllocationPending(&v.copyRoots)
}

// Capture only roots newly acquired by this concrete copy invocation. Cleanup
// uses the existing identity-checked numeral API and cannot claim older or
// replaced pending allocations belonging to another caller of the manager.
func (v *StackedPairVector) copyPair(dst, src *RationalPair) error {
	before, err := numeral.CaptureNumericAllocationRoots(v.manager.Z)
	if err != nil {
		return err
	}
	err = dst.Copy(v.manager, src)
	return numeral.JoinNumericErrors(err, v.copyRoots.After(v.manager.Z, before))
}
