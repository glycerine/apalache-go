package lp

// Source: src/util/lp/stacked_vector.h. Concrete retained instantiations only.

import "errors"

var ErrScope = errors.New("lp: invalid source scope count")

type stackedUintVectorChange struct {
	index uint32
	old   uint32
}

// StackedUintVector records literal replacement history and scope sizes. Zero value is
// empty; borrowed Values must not be modified. Do not copy after first use.
type StackedUintVector struct {
	values      []uint32
	changes     []stackedUintVectorChange
	vectorSizes []uint32
	changeSizes []uint32
}

func (v *StackedUintVector) Size() uint32     { return uint32(len(v.values)) }
func (v *StackedUintVector) Values() []uint32 { return v.values }
func (v *StackedUintVector) At(i uint32) (uint32, error) {
	if uint64(i) >= uint64(len(v.values)) {
		return 0, ErrBounds
	}
	return v.values[i], nil
}
func (v *StackedUintVector) Set(i uint32, value uint32) error {
	if uint64(i) >= uint64(len(v.values)) {
		return ErrBounds
	}
	if v.values[i] != value {
		v.changes = append(v.changes, stackedUintVectorChange{index: i, old: v.values[i]})
		v.values[i] = value
	}
	return nil
}
func (v *StackedUintVector) Append(value uint32) { v.values = append(v.values, value) }
func (v *StackedUintVector) IncreaseSizeByOne()  { v.values = append(v.values, 0) }
func (v *StackedUintVector) Push() {
	v.changeSizes = append(v.changeSizes, uint32(len(v.changes)))
	v.vectorSizes = append(v.vectorSizes, uint32(len(v.values)))
}
func (v *StackedUintVector) PeekSize(k uint32) (uint32, error) {
	if k == 0 || uint64(k) > uint64(len(v.vectorSizes)) {
		return 0, ErrScope
	}
	return v.vectorSizes[len(v.vectorSizes)-int(k)], nil
}
func (v *StackedUintVector) Pop(k uint32) error {
	if k == 0 || uint64(k) > uint64(len(v.vectorSizes)) {
		return ErrScope
	}
	firstScope := len(v.vectorSizes) - int(k)
	v.values = v.values[:v.vectorSizes[firstScope]]
	v.vectorSizes = v.vectorSizes[:firstScope]
	firstChange := v.changeSizes[firstScope]
	v.changeSizes = v.changeSizes[:firstScope]
	for j := len(v.changes); j > int(firstChange); {
		j--
		change := v.changes[j]
		if uint64(change.index) < uint64(len(v.values)) {
			v.values[change.index] = change.old
		}
	}
	v.changes = v.changes[:firstChange]
	return nil
}

type stackedIntVectorChange struct {
	index uint32
	old   int32
}

// StackedIntVector records literal replacement history and scope sizes. Zero value is
// empty; borrowed Values must not be modified. Do not copy after first use.
type StackedIntVector struct {
	values      []int32
	changes     []stackedIntVectorChange
	vectorSizes []uint32
	changeSizes []uint32
}

func (v *StackedIntVector) Size() uint32    { return uint32(len(v.values)) }
func (v *StackedIntVector) Values() []int32 { return v.values }
func (v *StackedIntVector) At(i uint32) (int32, error) {
	if uint64(i) >= uint64(len(v.values)) {
		return 0, ErrBounds
	}
	return v.values[i], nil
}
func (v *StackedIntVector) Set(i uint32, value int32) error {
	if uint64(i) >= uint64(len(v.values)) {
		return ErrBounds
	}
	if v.values[i] != value {
		v.changes = append(v.changes, stackedIntVectorChange{index: i, old: v.values[i]})
		v.values[i] = value
	}
	return nil
}
func (v *StackedIntVector) Append(value int32) { v.values = append(v.values, value) }
func (v *StackedIntVector) IncreaseSizeByOne() { v.values = append(v.values, 0) }
func (v *StackedIntVector) Push() {
	v.changeSizes = append(v.changeSizes, uint32(len(v.changes)))
	v.vectorSizes = append(v.vectorSizes, uint32(len(v.values)))
}
func (v *StackedIntVector) PeekSize(k uint32) (uint32, error) {
	if k == 0 || uint64(k) > uint64(len(v.vectorSizes)) {
		return 0, ErrScope
	}
	return v.vectorSizes[len(v.vectorSizes)-int(k)], nil
}
func (v *StackedIntVector) Pop(k uint32) error {
	if k == 0 || uint64(k) > uint64(len(v.vectorSizes)) {
		return ErrScope
	}
	firstScope := len(v.vectorSizes) - int(k)
	v.values = v.values[:v.vectorSizes[firstScope]]
	v.vectorSizes = v.vectorSizes[:firstScope]
	firstChange := v.changeSizes[firstScope]
	v.changeSizes = v.changeSizes[:firstScope]
	for j := len(v.changes); j > int(firstChange); {
		j--
		change := v.changes[j]
		if uint64(change.index) < uint64(len(v.values)) {
			v.values[change.index] = change.old
		}
	}
	v.changes = v.changes[:firstChange]
	return nil
}

type stackedColumnVectorChange struct {
	index uint32
	old   ColumnType
}

// StackedColumnVector records literal replacement history and scope sizes. Zero value is
// empty; borrowed Values must not be modified. Do not copy after first use.
type StackedColumnVector struct {
	values      []ColumnType
	changes     []stackedColumnVectorChange
	vectorSizes []uint32
	changeSizes []uint32
}

func (v *StackedColumnVector) Size() uint32         { return uint32(len(v.values)) }
func (v *StackedColumnVector) Values() []ColumnType { return v.values }
func (v *StackedColumnVector) At(i uint32) (ColumnType, error) {
	if uint64(i) >= uint64(len(v.values)) {
		return 0, ErrBounds
	}
	return v.values[i], nil
}
func (v *StackedColumnVector) Set(i uint32, value ColumnType) error {
	if uint64(i) >= uint64(len(v.values)) {
		return ErrBounds
	}
	if v.values[i] != value {
		v.changes = append(v.changes, stackedColumnVectorChange{index: i, old: v.values[i]})
		v.values[i] = value
	}
	return nil
}
func (v *StackedColumnVector) Append(value ColumnType) { v.values = append(v.values, value) }
func (v *StackedColumnVector) IncreaseSizeByOne()      { v.values = append(v.values, 0) }
func (v *StackedColumnVector) Push() {
	v.changeSizes = append(v.changeSizes, uint32(len(v.changes)))
	v.vectorSizes = append(v.vectorSizes, uint32(len(v.values)))
}
func (v *StackedColumnVector) PeekSize(k uint32) (uint32, error) {
	if k == 0 || uint64(k) > uint64(len(v.vectorSizes)) {
		return 0, ErrScope
	}
	return v.vectorSizes[len(v.vectorSizes)-int(k)], nil
}
func (v *StackedColumnVector) Pop(k uint32) error {
	if k == 0 || uint64(k) > uint64(len(v.vectorSizes)) {
		return ErrScope
	}
	firstScope := len(v.vectorSizes) - int(k)
	v.values = v.values[:v.vectorSizes[firstScope]]
	v.vectorSizes = v.vectorSizes[:firstScope]
	firstChange := v.changeSizes[firstScope]
	v.changeSizes = v.changeSizes[:firstScope]
	for j := len(v.changes); j > int(firstChange); {
		j--
		change := v.changes[j]
		if uint64(change.index) < uint64(len(v.values)) {
			v.values[change.index] = change.old
		}
	}
	v.changes = v.changes[:firstChange]
	return nil
}
