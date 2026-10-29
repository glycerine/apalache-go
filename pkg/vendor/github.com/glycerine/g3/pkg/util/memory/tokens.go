package memory

import (
	"github.com/glycerine/g3/pkg/arena"
)

func (b Block) OwnerID() uint64        { return b.origin }
func (b Block) Slot() uint32           { return b.slot }
func (b Block) Generation() uint32     { return b.generation }
func (o ObjectRef) OwnerID() uint64    { return o.owner }
func (o ObjectRef) Slot() uint32       { return o.slot }
func (o ObjectRef) Generation() uint32 { return o.generation }
func (r *Runtime) OwnerID() uint64 {
	if r == nil || r.self != r {
		return 0
	}
	return r.origin
}
func (r *Runtime) Budget() *arena.Budget {
	if r == nil || r.self != r {
		return nil
	}
	return r.config.Budget
}
func (r *Runtime) SizeTBits() uint8 {
	if r == nil || r.self != r {
		return 0
	}
	return r.config.SizeTBits
}
func (r *Runtime) CheckOpen() error { return r.open() }
func (a *SmallAllocator) RuntimeOwner() *Runtime {
	if a == nil || a.self != a {
		return nil
	}
	return a.config.Runtime
}
