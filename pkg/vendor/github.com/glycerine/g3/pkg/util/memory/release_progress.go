package memory

// Source: src/util/small_object_allocator.cpp:72-96; src/util/memory_manager.cpp.
// Small-allocator
// Source revision: Leonardo de Moura (leonardo) 2011-04-27; rewrote/simplified allocator.
// Explicit Go cleanup authority for completed source deallocation prefixes.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

const (
	runtimeReleaseHeader uint8 = iota + 1
	runtimeReleaseAccounted
	runtimeReleasePhysical
)

type runtimeReleaseState struct {
	serial, extent uint64
	generation     uint32
	stage          uint8
}
type allocatorReleaseState struct {
	serial, requested                       uint64
	raw                                     Block
	runtimeSerial                           uint64
	generation                              uint32
	runtimeStage                            uint8
	active, requestedAdjusted, pooledLinked bool
}

// The visible flags are observations, never authority to skip a source action.
// Authority is the private binding to the owner's actual retained directory.
type ReleaseProgress struct {
	owner, serial                                                   uint64
	slot, generation                                                uint32
	RequestedAdjusted, RawAccounted, PhysicalReleased, PooledLinked bool
}

func (m *Runtime) deallocateTracked(h *Block) (runtimeReleaseState, error) {
	if h == nil {
		return runtimeReleaseState{}, ErrRuntimeOwner
	}
	b, e := m.block(*h)
	if e != nil {
		return runtimeReleaseState{}, e
	}
	p := &b.release
	if p.stage == 0 || p.generation != h.generation {
		n, e := m.headerRead(b)
		if e != nil {
			return runtimeReleaseState{}, e
		}
		if m.nextReleaseSerial == math.MaxUint64 {
			return runtimeReleaseState{}, arena.ErrSlotLimit
		}
		m.nextReleaseSerial++
		*p = runtimeReleaseState{serial: m.nextReleaseSerial, extent: n, generation: h.generation, stage: runtimeReleaseHeader}
	}
	if p.stage == runtimeReleaseHeader {
		if e = m.transport(m.accountingOwner().beginExtentDeallocation(&m.lane, p.extent, m.config.SizeTBits, m.config.ThreadLocal)); e != nil {
			return *p, e
		}
		p.stage = runtimeReleaseAccounted
	}
	m.config.Budget.Release(uint64(len(b.data)) + uint64(len(b.initialized))*8)
	b.data = nil
	b.initialized = nil
	b.live = false
	m.live--
	if b.generation != math.MaxUint32 {
		b.generation++
	}
	*h = Block{}
	p.stage = runtimeReleasePhysical
	return *p, m.transport(m.accountingOwner().finishDeallocation(&m.lane, m.config.ThreadLocal))
}

func (a *SmallAllocator) releaseSnapshot(r ObjectRef, p *allocatorReleaseState) ReleaseProgress {
	return ReleaseProgress{owner: a.origin, slot: r.slot, generation: p.generation, serial: p.serial,
		RequestedAdjusted: p.requestedAdjusted, RawAccounted: p.runtimeStage >= runtimeReleaseAccounted,
		PhysicalReleased: p.runtimeStage == runtimeReleasePhysical, PooledLinked: p.pooledLinked}
}

// DeallocateTracked shares the sole normal deallocation body with Deallocate.
// A genuine pending invocation resumes its stored phase; a completed normal
// pooled release starts a new invocation, preserving source duplicate free.
func (a *SmallAllocator) DeallocateTracked(n uint64, r ObjectRef) (ReleaseProgress, error) {
	if e := a.entryOpen(); e != nil {
		return ReleaseProgress{}, e
	}
	// Source size_t argument conversion precedes the body zero shortcut.
	n = a.counter(n)
	if n == 0 {
		return ReleaseProgress{}, nil
	}
	if a.rawRelease.active {
		return ReleaseProgress{}, ErrAllocatorPending
	}
	if r.owner == a.origin && r.slot != 0 && uint64(r.slot) <= uint64(len(a.locations)) {
		loc := &a.locations[r.slot-1]
		if loc.generation == r.generation && loc.release.active && loc.release.generation == r.generation {
			if loc.release.requested != n {
				return ReleaseProgress{}, ErrAllocatorOwner
			}
			return a.resumeRelease(r, loc)
		}
	}
	if a.nextReleaseSerial == math.MaxUint64 {
		return ReleaseProgress{}, arena.ErrSlotLimit
	}
	a.nextReleaseSerial++
	serial := a.nextReleaseSerial
	if a.bypass() {
		loc, e := a.object(r)
		if e != nil {
			return ReleaseProgress{}, e
		}
		loc.release = allocatorReleaseState{serial: serial, requested: n, raw: loc.raw, generation: r.generation, active: true}
		return a.resumeRelease(r, loc)
	}
	if e := a.sourceAssert(a.allocSize >= n); e != nil {
		return ReleaseProgress{}, e
	}
	if e := a.sourceAssert(r.slot != 0); e != nil {
		return ReleaseProgress{}, e
	}
	a.allocSize = a.counter(a.allocSize - n)
	// Source performs requested subtraction before validating pointer provenance.
	// An invalid pointer produces observations without cleanup authority.
	observed := ReleaseProgress{RequestedAdjusted: true}
	loc, e := a.location(r.slot)
	if e != nil {
		return observed, e
	}
	if r.owner != a.origin || loc.generation != r.generation ||
		(!loc.live && !(loc.kind == smallObjectPooled && loc.free)) ||
		(loc.kind != smallObjectPooled && loc.kind != smallObjectDirect) {
		return observed, ErrAllocatorOwner
	}
	loc.release = allocatorReleaseState{serial: serial, requested: n, raw: loc.raw, generation: r.generation, active: true, requestedAdjusted: true}
	return a.resumeRelease(r, loc)
}

func (a *SmallAllocator) resumeRelease(r ObjectRef, loc *smallObjectLocation) (ReleaseProgress, error) {
	p := &loc.release
	if a.bypass() || p.requested >= 256-a.pointerBytes() {
		if !a.bypass() && loc.offset != 0 {
			return a.releaseSnapshot(r, p), arena.ErrBounds
		}
		h := loc.raw
		progress, e := a.config.Runtime.deallocateTracked(&h)
		p.runtimeSerial = progress.serial
		p.runtimeStage = progress.stage
		loc.raw = h
		if h == (Block{}) {
			loc.live = false
			p.active = false
		}
		return a.releaseSnapshot(r, p), e
	}
	slot := uint32((p.requested + a.pointerBytes() - 1) / a.pointerBytes())
	if e := a.sourceAssert(slot > 0); e != nil {
		return a.releaseSnapshot(r, p), e
	}
	if e := a.sourceAssert(slot < a.slots()); e != nil {
		return a.releaseSnapshot(r, p), e
	}
	if slot == 0 || slot >= a.slots() {
		return a.releaseSnapshot(r, p), arena.ErrBounds
	}
	if e := smallObjectWriteWord(a.config.Runtime, loc.raw, uint64(loc.offset), a.pointerBytes(), uint64(a.free[slot])); e != nil {
		return a.releaseSnapshot(r, p), e
	}
	a.free[slot] = r.slot
	loc.live = false
	loc.free = true
	p.pooledLinked = true
	p.active = false
	return a.releaseSnapshot(r, p), nil
}

// CompleteRelease accepts only the private binding matching real retained
// owner state. Caller flags are ignored and refreshed from that state.
func (a *SmallAllocator) CompleteRelease(n uint64, r ObjectRef, progress *ReleaseProgress) error {
	if e := a.open(); e != nil {
		return e
	}
	// The retained invocation stores the actual source size_t argument.
	n = a.counter(n)
	if progress == nil || r.owner != a.origin || progress.owner != a.origin || progress.slot != r.slot || progress.generation != r.generation {
		return ErrAllocatorOwner
	}
	loc, e := a.location(r.slot)
	if e != nil {
		return e
	}
	p := &loc.release
	if loc.generation != r.generation || p.generation != r.generation || p.serial == 0 || p.serial != progress.serial || p.requested != n {
		return ErrAllocatorOwner
	}
	if p.active {
		next, e := a.resumeRelease(r, loc)
		*progress = next
		return e
	}
	*progress = a.releaseSnapshot(r, p)
	return nil
}
