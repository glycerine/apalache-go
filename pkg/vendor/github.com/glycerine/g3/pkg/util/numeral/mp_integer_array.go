package numeral

// Source: src/util/mpz.cpp:845-922; src/util/mpzzp.h:217-220.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// A call-local concrete borrowed view, never a duplicated owning numeral array.
// The slice route retains the existing API's aliasing; the codec route loads
// only the source-reached numeral field from an actual Runtime backing.
type IntegerArrayView struct {
	Allocator      *umemory.SmallAllocator
	Object         umemory.ObjectRef
	Values         []Int
	Runtime        *umemory.Runtime
	Raw            umemory.Block
	Count          uint32
	Offset, Stride uint64
}

func (v IntegerArrayView) At(i uint32) (Int, error) {
	if i >= v.Count {
		return Int{}, arena.ErrBounds
	}
	if v.Runtime == nil && v.Allocator == nil {
		if uint64(i) >= uint64(len(v.Values)) {
			return Int{}, arena.ErrBounds
		}
		return v.Values[i], nil
	}
	if v.Stride != 0 && uint64(i) > (math.MaxUint64-v.Offset)/v.Stride {
		return Int{}, arena.ErrBounds
	}
	o := v.Offset + uint64(i)*v.Stride
	if o > math.MaxUint64-12 {
		return Int{}, arena.ErrBounds
	}
	var value, lo, hi uint32
	var e error
	if v.Allocator != nil {
		value, e = v.Allocator.ReadUint32(v.Object, o)
	} else {
		value, e = v.Runtime.ReadUint32(v.Raw, o)
	}
	if e != nil {
		return Int{}, e
	}
	if v.Allocator != nil {
		lo, e = v.Allocator.ReadUint32(v.Object, o+4)
	} else {
		lo, e = v.Runtime.ReadUint32(v.Raw, o+4)
	}
	if e != nil {
		return Int{}, e
	}
	if v.Allocator != nil {
		hi, e = v.Allocator.ReadUint32(v.Object, o+8)
	} else {
		hi, e = v.Runtime.ReadUint32(v.Raw, o+8)
	}
	if e != nil {
		return Int{}, e
	}
	ref := uint64(lo) | uint64(hi)<<32
	return Int{Val: int32(value), Cell: NumericCellRef(ref)}, nil
}
func (v IntegerArrayView) Reference(i uint32, local *Int) (*Int, error) {
	if i >= v.Count {
		return nil, arena.ErrBounds
	}
	if v.Runtime == nil && v.Allocator == nil {
		if uint64(i) >= uint64(len(v.Values)) {
			return nil, arena.ErrBounds
		}
		return &v.Values[i], nil
	}
	a, e := v.At(i)
	if e != nil {
		return nil, e
	}
	*local = a
	return local, nil
}
func (m *IntegerManager) GCDManyIndexed(as IntegerArrayView, g *Int) error {
	return m.gcdManyIndexedWithAllocationContext(as, g, integerAllocationInvocationContext{})
}
func (m *IntegerManager) gcdManyIndexedWithAllocationContext(as IntegerArrayView, g *Int, context integerAllocationInvocationContext) error {
	if e := m.check(g); e != nil {
		return e
	}
	var a, b Int
	switch as.Count {
	case 0:
		return m.Reset(g)
	case 1:
		pa, e := as.referenceWithAllocatorOwner(m.Memory, 0, &a)
		if e != nil {
			return e
		}
		if e = m.copyFromOwnerWithAllocationContext(g, m, pa, nil, context); e != nil {
			return e
		}
		return m.absWithAllocationContext(g, context)
	}
	pa, e := as.referenceWithAllocatorOwner(m.Memory, 0, &a)
	if e != nil {
		return e
	}
	pb, e := as.referenceWithAllocatorOwner(m.Memory, 1, &b)
	if e != nil {
		return e
	}
	if e = m.gcdWithAllocationContext(pa, pb, g, context); e != nil {
		return e
	}
	for i := uint32(2); i < as.Count; i++ {
		if isOneInteger(g) {
			return nil
		}
		pa, e = as.referenceWithAllocatorOwner(m.Memory, i, &a)
		if e != nil {
			return e
		}
		if e = m.gcdWithAllocationContext(g, pa, g, context); e != nil {
			return e
		}
	}
	return nil
}
func (m *IntegerModularManager) GCDManyIndexed(as IntegerArrayView, g *Int) error {
	if e := m.check(); e != nil {
		return e
	}
	if e := m.manager.GCDManyIndexed(as, g); e != nil {
		return e
	}
	return m.AssertNormalized(g)
}

// checkedAt validates the borrowed cohort only when this numeral is reached.
func (v IntegerArrayView) checkedAt(owner *umemory.Runtime, i uint32) (Int, error) {
	if v.Allocator != nil && v.Allocator.RuntimeOwner() != owner {
		return Int{}, umemory.ErrRuntimeOwner
	}
	if v.Runtime != nil && v.Runtime != owner {
		return Int{}, umemory.ErrRuntimeOwner
	}
	return v.At(i)
}
func (v IntegerArrayView) sameRuntimeElement(owner *umemory.Runtime, raw umemory.Block, offset uint64, i uint32) bool {
	if i >= v.Count || v.Allocator != nil || v.Runtime != owner || v.Raw != raw {
		return false
	}
	if v.Stride != 0 && uint64(i) > (math.MaxUint64-v.Offset)/v.Stride {
		return false
	}
	return v.Offset+uint64(i)*v.Stride == offset
}
func (v IntegerArrayView) store(i uint32, a Int) error {
	if i >= v.Count {
		return arena.ErrBounds
	}
	if v.Runtime == nil && v.Allocator == nil {
		if uint64(i) >= uint64(len(v.Values)) {
			return arena.ErrBounds
		}
		v.Values[i] = a
		return nil
	}
	if v.Stride != 0 && uint64(i) > (math.MaxUint64-v.Offset)/v.Stride {
		return arena.ErrBounds
	}
	o := v.Offset + uint64(i)*v.Stride
	if o > math.MaxUint64-12 {
		return arena.ErrBounds
	}
	words := [3]uint32{uint32(a.Val), uint32(a.Cell), uint32(uint64(a.Cell) >> 32)}
	for j, w := range words {
		var e error
		if v.Allocator != nil {
			e = v.Allocator.WriteUint32(v.Object, o+uint64(j)*4, w)
		} else {
			e = v.Runtime.WriteUint32(v.Raw, o+uint64(j)*4, w)
		}
		if e != nil {
			return e
		}
	}
	return nil
}

// Only a source-reached coefficient requires the borrowed object allocator.
func (v IntegerArrayView) referenceWithAllocatorOwner(owner *umemory.Runtime, i uint32, local *Int) (*Int, error) {
	if v.Runtime != nil && v.Runtime != owner {
		return nil, umemory.ErrRuntimeOwner
	}
	if v.Allocator != nil && v.Allocator.RuntimeOwner() != owner {
		return nil, umemory.ErrRuntimeOwner
	}
	return v.Reference(i, local)
}

// Store writes the borrowed source representation without acquiring a second
// numeral owner. The enclosing source aggregate controls ownership and release.
func (v IntegerArrayView) Store(i uint32, a Int) error { return v.store(i, a) }
func (v IntegerArrayView) CheckedAt(owner *umemory.Runtime, i uint32) (Int, error) {
	return v.checkedAt(owner, i)
}
func (v IntegerArrayView) SameRuntimeElement(owner *umemory.Runtime, raw umemory.Block, offset uint64, i uint32) bool {
	return v.sameRuntimeElement(owner, raw, offset, i)
}
