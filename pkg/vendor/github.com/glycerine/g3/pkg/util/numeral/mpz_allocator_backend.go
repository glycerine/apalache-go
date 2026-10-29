package numeral

// Source: src/util/mpz.h; src/util/mpz.cpp (_MP_INTERNAL, digit32).
// Actual mpz.h cell allocation and object-bounded initialized-byte access.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// Go safety metadata, to be embedded/charged in the numeric descriptor.
// The actual source payload remains one contiguous 8+4*capacity object.

type SourceCellBacking struct {
	Object                    umemory.ObjectRef
	Raw                       umemory.Block
	Offset, extent, Requested uint64
}

func sourceMPZCellBytes(capacity uint32) uint64 { return uint64(uint32(8) + uint32(4)*capacity) }

// requested retains the result of the source unsigned32 cell-byte expression,
// widened only for the Go storage API. It cannot legitimately exceed uint32.
// Guard corrupt retained metadata before source size_t conversion can erase it.
func (c SourceCellBacking) deallocate(a *umemory.SmallAllocator) (umemory.ReleaseProgress, error) {
	if c.Requested > uint64(^uint32(0)) {
		return umemory.ReleaseProgress{}, arena.ErrBounds
	}
	return a.DeallocateTracked(c.Requested, c.Object)
}

// The returned scalar root is nonzero on a completed allocation even if the
// capacity write fails. Its caller must retain it before descriptor preparation.
func allocateSourceCell(a *umemory.SmallAllocator, capacity uint32) (SourceCellBacking, error) {
	return allocateSourceCellWithOutcome(a, capacity, nil)
}
func allocateSourceCellWithOutcome(a *umemory.SmallAllocator, capacity uint32, outcome *umemory.SmallAllocatorAllocationOutcome) (SourceCellBacking, error) {
	var c SourceCellBacking
	c.Requested = sourceMPZCellBytes(capacity)
	object, e := a.AllocateWithOutcome(c.Requested, outcome)
	c.Object = object
	if e != nil {
		return c, e
	}
	if object == (umemory.ObjectRef{}) {
		return c, arena.ErrBounds
	} // actual null header access after size0 allocation
	c.extent, e = a.ObjectExtent(object)
	if e != nil {
		return c, e
	}
	c.Raw, c.Offset, e = a.RawLocation(object)
	if e != nil {
		return c, e
	}
	return c, c.writeWord(a, 4, capacity) // allocate() initializes capacity ONLY
}
func (c SourceCellBacking) location(a *umemory.SmallAllocator, offset uint64) (umemory.Block, uint64, error) {
	// Revalidate object generation before borrowing the underlying pooled chunk.
	// A raw chunk alone would otherwise authorize reads after cell recycle.
	raw, start, e := a.RawLocation(c.Object)
	if e != nil {
		return umemory.Block{}, 0, e
	}
	extent, e := a.ObjectExtent(c.Object)
	if e != nil {
		return umemory.Block{}, 0, e
	}
	if offset > extent || extent-offset < 4 {
		return umemory.Block{}, 0, arena.ErrBounds
	}
	if raw != c.Raw || start != c.Offset {
		return umemory.Block{}, 0, umemory.ErrAllocatorOwner
	}
	if start > ^uint64(0)-offset {
		return umemory.Block{}, 0, arena.ErrBounds
	}
	return raw, start + offset, nil
}
func (c SourceCellBacking) ReadWord(a *umemory.SmallAllocator, offset uint64) (uint32, error) {
	raw, at, e := c.location(a, offset)
	if e != nil {
		return 0, e
	}
	return a.RuntimeOwner().ReadUint32(raw, at)
}
func (c SourceCellBacking) writeWord(a *umemory.SmallAllocator, offset uint64, word uint32) error {
	raw, at, e := c.location(a, offset)
	if e != nil {
		return e
	}
	return a.RuntimeOwner().WriteUint32(raw, at, word)
}
