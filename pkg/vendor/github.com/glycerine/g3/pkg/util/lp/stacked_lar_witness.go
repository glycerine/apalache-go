package lp

// Source: src/util/lp/stacked_vector.h, live ul_pair specialization.

type stackedLARWitnessChange struct {
	index uint32
	old   LARBoundWitnesses
}

// StackedLARWitnessVector owns native witness values and their replacement
// history. Values returns a read-only borrow; use Set for scoped replacement.
type StackedLARWitnessVector struct {
	values                   []LARBoundWitnesses
	changes                  []stackedLARWitnessChange
	vectorSizes, changeSizes []uint32
}

func (v *StackedLARWitnessVector) Size() uint32                { return uint32(len(v.values)) }
func (v *StackedLARWitnessVector) Values() []LARBoundWitnesses { return v.values }
func (v *StackedLARWitnessVector) At(j uint32) (LARBoundWitnesses, error) {
	if uint64(j) >= uint64(len(v.values)) {
		return LARBoundWitnesses{}, ErrBounds
	}
	return v.values[j], nil
}
func (v *StackedLARWitnessVector) Set(j uint32, value LARBoundWitnesses) error {
	if uint64(j) >= uint64(len(v.values)) {
		return ErrBounds
	}
	if v.values[j] != value {
		v.changes = append(v.changes, stackedLARWitnessChange{j, v.values[j]})
		v.values[j] = value
	}
	return nil
}
func (v *StackedLARWitnessVector) Append(value LARBoundWitnesses) { v.values = append(v.values, value) }
func (v *StackedLARWitnessVector) Push() {
	v.changeSizes = append(v.changeSizes, uint32(len(v.changes)))
	v.vectorSizes = append(v.vectorSizes, uint32(len(v.values)))
}
func (v *StackedLARWitnessVector) PeekSize(k uint32) (uint32, error) {
	if k == 0 || uint64(k) > uint64(len(v.vectorSizes)) {
		return 0, ErrScope
	}
	return v.vectorSizes[len(v.vectorSizes)-int(k)], nil
}
func (v *StackedLARWitnessVector) Pop(k uint32) error {
	if k == 0 || uint64(k) > uint64(len(v.vectorSizes)) {
		return ErrScope
	}
	i := len(v.vectorSizes) - int(k)
	v.values = v.values[:v.vectorSizes[i]]
	v.vectorSizes = v.vectorSizes[:i]
	first := v.changeSizes[i]
	v.changeSizes = v.changeSizes[:i]
	for j := len(v.changes); j > int(first); {
		j--
		c := v.changes[j]
		if uint64(c.index) < uint64(len(v.values)) {
			v.values[c.index] = c.old
		}
	}
	v.changes = v.changes[:first]
	return nil
}
