package diagnostic

// Source: src/util/small_object_allocator.h; src/util/small_object_allocator.cpp; src/util/debug.h:63.
// Charged main adapter; all allocation algorithms live solely in memory.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	utext "github.com/glycerine/g3/pkg/util/text"
)

var ErrSmallObjectOwner = umemory.ErrAllocatorOwner
var ErrSmallObjectAssertion = umemory.ErrAllocatorAssertion
var ErrSmallObjectUnreachable = umemory.ErrAllocatorUnreachable
var ErrSmallObjectPending = umemory.ErrAllocatorPending
var ErrSmallObjectNonterminating = umemory.ErrAllocatorNonterminating

type utilSmallObjectProfile = umemory.SmallAllocatorProfile
type utilSmallObjectRef = umemory.ObjectRef
type smallObjectStats = umemory.AllocatorStats
type utilSmallObjectConfig struct {
	Debug                *DebugContext
	Trace                *TraceTags
	Runtime              *umemory.Runtime
	Profile              utilSmallObjectProfile
	ID                   string
	IDPresent, IDNull    bool
	BorrowedID           *arena.BytesRegion
	IDRef                utext.RegionCString
	Diagnostics, Verbose *utext.NumericTextStream
	Verbosity            uint32
	VerboseContext       *VerboseContext
}
type smallObjectIOBridge struct{ config utilSmallObjectConfig }
type utilSmallObjectAllocator struct {
	self                 *utilSmallObjectAllocator
	core                 *umemory.SmallAllocator
	io                   *smallObjectIOBridge
	budget               *arena.Budget
	headerBytes, ioBytes uint64
}

func newUtilSmallObjectAllocator(c utilSmallObjectConfig) (*utilSmallObjectAllocator, error) {
	if c.Runtime == nil {
		return nil, ErrSmallObjectOwner
	}
	if e := c.Runtime.CheckOpen(); e != nil {
		return nil, e
	}
	if c.Profile.PointerBits == 0 {
		c.Profile.PointerBits = c.Runtime.SizeTBits()
	}
	if (c.Profile.PointerBits != 32 && c.Profile.PointerBits != 64) || c.Profile.PointerBits != c.Runtime.SizeTBits() {
		return nil, arena.ErrConfig
	}
	b := c.Runtime.Budget()
	h := uint64(reflect.TypeOf(utilSmallObjectAllocator{}).Size())
	i := uint64(reflect.TypeOf(smallObjectIOBridge{}).Size())
	if e := b.Reserve(h); e != nil {
		return nil, e
	}
	if e := b.Reserve(i); e != nil {
		b.Release(h)
		return nil, e
	}
	if c.ID == "" && !c.IDPresent {
		c.ID = "unknown"
	}
	io := &smallObjectIOBridge{config: c}
	mc := umemory.SmallAllocatorConfig{Runtime: c.Runtime, Profile: c.Profile, IO: io, Verbosity: c.Verbosity}
	mc.ChunkTrace = NewMemoryChunkTrace(c.Trace, c.Runtime)
	if c.Debug != nil {
		mc.Assertions = c.Debug
	}
	if c.VerboseContext != nil {
		mc.VerboseControl = c.VerboseContext
	}
	core, e := umemory.NewSmallAllocator(mc)
	if e != nil {
		b.Release(i + h)
		return nil, e
	}
	a := &utilSmallObjectAllocator{core: core, io: io, budget: b, headerBytes: h, ioBytes: i}
	a.self = a
	return a, nil
}
func (a *utilSmallObjectAllocator) valid() error {
	if a == nil || a.self != a || a.core == nil {
		return ErrSmallObjectOwner
	}
	return nil
}
func (a *utilSmallObjectAllocator) Allocate(n uint64) (utilSmallObjectRef, error) {
	if e := a.valid(); e != nil {
		return utilSmallObjectRef{}, e
	}
	return a.core.Allocate(n)
}
func (a *utilSmallObjectAllocator) Deallocate(n uint64, r utilSmallObjectRef) error {
	if e := a.valid(); e != nil {
		return e
	}
	return a.core.Deallocate(n, r)
}
func (a *utilSmallObjectAllocator) Read(r utilSmallObjectRef, n uint64) (byte, error) {
	if e := a.valid(); e != nil {
		return 0, e
	}
	return a.core.Read(r, n)
}
func (a *utilSmallObjectAllocator) Write(r utilSmallObjectRef, n uint64, v byte) error {
	if e := a.valid(); e != nil {
		return e
	}
	return a.core.Write(r, n, v)
}
func (a *utilSmallObjectAllocator) RawLocation(r utilSmallObjectRef) (umemory.Block, uint64, error) {
	if e := a.valid(); e != nil {
		return umemory.Block{}, 0, e
	}
	return a.core.RawLocation(r)
}
func (a *utilSmallObjectAllocator) AllocationSize() (uint64, error) {
	if e := a.valid(); e != nil {
		return 0, e
	}
	return a.core.AllocationSize()
}
func (a *utilSmallObjectAllocator) WastedSize() (uint64, error) {
	if e := a.valid(); e != nil {
		return 0, e
	}
	return a.core.WastedSize()
}
func (a *utilSmallObjectAllocator) NumFreeObjects() (uint64, error) {
	if e := a.valid(); e != nil {
		return 0, e
	}
	return a.core.NumFreeObjects()
}
func (a *utilSmallObjectAllocator) Consolidate() error {
	if e := a.valid(); e != nil {
		return e
	}
	return a.core.Consolidate()
}
func (a *utilSmallObjectAllocator) Reset() error {
	if e := a.valid(); e != nil {
		return e
	}
	return a.core.Reset()
}
func (a *utilSmallObjectAllocator) CleanupPending() error {
	if e := a.valid(); e != nil {
		return e
	}
	return a.core.CleanupPending()
}
func (a *utilSmallObjectAllocator) CleanupChunks() error {
	if e := a.valid(); e != nil {
		return e
	}
	return a.core.CleanupChunks()
}
func (a *utilSmallObjectAllocator) Close() error {
	if e := a.valid(); e != nil {
		return e
	}
	e := a.core.Close()
	if a.core.Stats().OwnerBytes == 0 && a.headerBytes != 0 {
		a.budget.Release(a.headerBytes + a.ioBytes)
		a.headerBytes = 0
		a.ioBytes = 0
		a.io = nil
	}
	return e
}
func (a *utilSmallObjectAllocator) Stats() smallObjectStats {
	if a.valid() != nil {
		return smallObjectStats{}
	}
	s := a.core.Stats()
	s.OwnerBytes += a.headerBytes + a.ioBytes
	s.TotalBytes += a.headerBytes + a.ioBytes
	return s
}
func (a *utilSmallObjectAllocator) PlacementDelete(r utilSmallObjectRef) error {
	if e := a.valid(); e != nil {
		return e
	}
	return a.core.PlacementDelete(r)
}
func (a *utilSmallObjectAllocator) PlacementDeleteArray(r utilSmallObjectRef) error {
	if e := a.valid(); e != nil {
		return e
	}
	return a.core.PlacementDeleteArray(r)
}
