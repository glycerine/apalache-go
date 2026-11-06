package memory

// Source: src/util/small_object_allocator.cpp:72-96; src/util/small_object_allocator.h.
// Source revision: Leonardo de Moura (leonardo) 2011-04-27; rewrote/simplified allocator.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

// RawObjectPointer is a concrete borrowed source-pointer binding. It grants no
// ownership of a donor chunk and performs no ObjectRef registry lookup.
type RawObjectPointer struct {
	Runtime *Runtime
	Block   Block
	Offset  uint64
}
type rawAllocatorReleaseState struct {
	raw                                Block
	offset, requested, serial          uint64
	stage                              uint8
	active, requestedAdjusted, aborted bool
}

// DeallocateRawTracked follows receiver accounting then the actual raw pointer
// branch. Retrying an interrupted invocation is an explicit Go continuation;
// it does not repeat a completed source subtraction. The pointer is not changed.
func (a *SmallAllocator) DeallocateRawTracked(n uint64, p RawObjectPointer) (ReleaseProgress, error) {
	if a == nil || a.self != a {
		return ReleaseProgress{}, ErrAllocatorOwner
	}
	if a.closed || a.destructorDone {
		return ReleaseProgress{}, arena.ErrClosed
	}
	if e := a.config.Runtime.open(); e != nil {
		return ReleaseProgress{}, e
	}
	// Actual source size_t argument conversion precedes every body branch.
	n = a.counter(n)
	if n == 0 {
		return ReleaseProgress{}, nil
	}
	s := &a.rawRelease
	if s.active {
		if p.Runtime != a.config.Runtime || p.Block != s.raw || p.Offset != s.offset || n != s.requested {
			return a.rawReleaseSnapshot(), ErrAllocatorPending
		}
		return a.resumeRawRelease()
	}
	// A real stopped ObjectRef release already published its requested-count
	// frontier. A raw alias must not create a second invocation over that receipt.
	// This matches an active action binding, not inferred allocation ownership.
	for i := range a.locations {
		l := &a.locations[i]
		if l.release.active && l.release.raw == p.Block && uint64(l.offset) == p.Offset && p.Runtime == a.config.Runtime {
			return ReleaseProgress{}, ErrAllocatorPending
		}
	}
	if a.nextReleaseSerial == math.MaxUint64 {
		return ReleaseProgress{}, arena.ErrSlotLimit
	}
	a.nextReleaseSerial++
	if !a.bypass() {
		if e := a.sourceAssert(a.allocSize >= n); e != nil {
			return ReleaseProgress{}, e
		}
		if e := a.sourceAssert(p.Block != (Block{})); e != nil {
			return ReleaseProgress{}, e
		}
		a.allocSize = a.counter(a.allocSize - n)
	}
	*s = rawAllocatorReleaseState{raw: p.Block, offset: p.Offset, requested: n, serial: a.nextReleaseSerial, active: true, requestedAdjusted: !a.bypass()}
	// The source pointer/provider is reached only after receiver accounting.
	if p.Runtime != a.config.Runtime {
		s.active = false
		return a.rawReleaseSnapshot(), ErrRuntimeOwner
	}
	return a.resumeRawRelease()
}
func (a *SmallAllocator) rawReleaseSnapshot() ReleaseProgress {
	s := &a.rawRelease
	return ReleaseProgress{owner: a.origin, serial: s.serial, RequestedAdjusted: s.requestedAdjusted, RawAccounted: s.stage == runtimeReleaseAccounted || s.stage == runtimeReleasePhysical, PhysicalReleased: s.stage == runtimeReleasePhysical, PooledLinked: s.stage == 4}
}
func (a *SmallAllocator) resumeRawRelease() (ReleaseProgress, error) {
	s := &a.rawRelease
	if a.bypass() || s.requested >= 256-a.pointerBytes() {
		if s.offset != 0 {
			return a.rawReleaseSnapshot(), arena.ErrBounds
		}
		h := s.raw
		progress, e := a.config.Runtime.deallocateTracked(&h)
		s.stage = progress.stage
		if h == (Block{}) {
			s.active = false
		}
		return a.rawReleaseSnapshot(), e
	}
	slot := uint32((s.requested + a.pointerBytes() - 1) / a.pointerBytes())
	if e := a.sourceAssert(slot > 0); e != nil {
		return a.rawReleaseSnapshot(), e
	}
	if e := a.sourceAssert(slot < a.slots()); e != nil {
		return a.rawReleaseSnapshot(), e
	}
	if slot == 0 || slot >= a.slots() {
		return a.rawReleaseSnapshot(), arena.ErrBounds
	}
	// The extra concrete locator is a Go provenance sidecar, not a source malloc.
	// Its reservation occurs after the real source requested-count publication.
	// Canonicalize the scalar pointer codec, not allocation ownership. The
	// supplied pointer still reaches the real Runtime write below before linking.
	id := uint32(0)
	for i := range a.locations {
		l := &a.locations[i]
		if l.kind == smallObjectPooled && l.raw == s.raw && uint64(l.offset) == s.offset {
			id = uint32(i + 1)
			break
		}
	}
	if id == 0 {
		if e := a.reserveLocations(1); e != nil {
			return a.rawReleaseSnapshot(), e
		}
	}
	if e := smallObjectWriteWord(a.config.Runtime, s.raw, s.offset, a.pointerBytes(), uint64(a.free[slot])); e != nil {
		return a.rawReleaseSnapshot(), e
	}
	if s.offset > math.MaxUint32 {
		return a.rawReleaseSnapshot(), arena.ErrBounds
	}
	// No chunk ownership moves. This record borrows the actual donor payload.
	if id == 0 {
		id = a.installLocation(smallObjectLocation{raw: s.raw, address: s.offset, requested: s.requested, offset: uint32(s.offset), extent: slot * uint32(a.pointerBytes()), kind: smallObjectPooled, free: true})
	} else {
		l := &a.locations[id-1]
		l.requested = s.requested
		l.extent = slot * uint32(a.pointerBytes())
		l.live = false
		l.free = true
		l.release = allocatorReleaseState{}
	}
	a.free[slot] = id
	s.stage = 4
	s.active = false
	return a.rawReleaseSnapshot(), nil
}

// RawReleasePending names the stopped pointer; it is a borrow, not authority to
// sweep donor allocations or to free a donor page through receiver Close.
func (a *SmallAllocator) RawReleasePending() (RawObjectPointer, uint64, bool) {
	if a == nil || a.self != a {
		return RawObjectPointer{}, 0, false
	}
	s := &a.rawRelease
	return RawObjectPointer{a.config.Runtime, s.raw, s.offset}, s.requested, s.active
}

// AbortRawInvocation is an identity-only owning abort of this receiver's
// stopped invocation metadata. The input raw pointer remains caller-owned;
// its donor page is never freed, adopted or inferred from locator records.
// Actual source count/stage observations remain available after acknowledgment.
func (a *SmallAllocator) AbortRawInvocation() (RawObjectPointer, ReleaseProgress, error) {
	if a == nil || a.self != a {
		return RawObjectPointer{}, ReleaseProgress{}, ErrAllocatorOwner
	}
	s := &a.rawRelease
	p := RawObjectPointer{a.config.Runtime, s.raw, s.offset}
	if s.active {
		s.active = false
		s.aborted = true
	}
	return p, a.rawReleaseSnapshot(), nil
}
func (a *SmallAllocator) RawReleaseAborted() bool {
	return a != nil && a.self == a && a.rawRelease.aborted
}
