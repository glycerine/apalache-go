package memory

// Source: src/util/small_object_allocator.h; src/util/small_object_allocator.cpp; explicit scalar pointer codec.

import (
	"errors"
	"math"
	"reflect"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"
)

var ErrAllocatorOwner = errors.New("g3: invalid small-object allocator owner or object")
var ErrAllocatorAssertion = errors.New("g3: small-object allocator source assertion")
var ErrAllocatorUnreachable = errors.New("g3: unreachable small-object placement delete")
var ErrAllocatorPending = errors.New("g3: pending small-object allocation or cleanup")
var ErrAllocatorNonterminating = errors.New("g3: cyclic small-object source traversal")
var smallObjectOrigins atomic.Uint64

type SmallAllocatorProfile struct {
	PointerBits    uint8
	Debug, Windows bool
}
type SmallAllocatorConfig struct {
	Assertions     AllocatorAssertionControl
	VerboseControl AllocatorVerboseControl
	Runtime        *Runtime
	Profile        SmallAllocatorProfile
	IO             AllocatorIO
	ChunkTrace     SmallAllocatorChunkTrace
	Verbosity      uint32
}

// SmallAllocatorChunkTrace is the concrete memory::alloc(chunk)/dealloc(chunk)
// diagnostic boundary. The address is the allocator's stable logical address.
type SmallAllocatorChunkTrace interface {
	AllocatedChunk(address uint64)
	DeallocatedChunk(address uint64, sourceLine int)
}
type ObjectRef struct {
	owner, address   uint64
	slot, generation uint32
}

func (r ObjectRef) LogicalAddress() uint64 { return r.address }

const (
	smallObjectChunk  uint8 = 1
	smallObjectPooled uint8 = 2
	smallObjectDirect uint8 = 3
)

// This concrete locator is pointer-free. Source links live in actual payload;
// this ledger provides checked identity, physical ownership and source addresses.
type smallObjectLocation struct {
	raw                               Block
	address, requested                uint64
	offset, extent, chunk, generation uint32
	kind                              uint8
	live, free, retired               bool
	release                           allocatorReleaseState
}
type SmallAllocator struct {
	allocationInvocation                smallAllocatorAllocationScope
	self                                *SmallAllocator
	config                              SmallAllocatorConfig
	origin, header, allocSize, sequence uint64
	locations                           []smallObjectLocation
	chunks, free                        [64]uint32
	pending                             Block
	pendingPartial                      bool
	cleanupClass, cleanupCurrent        uint32
	cleanupMode                         uint8
	closed                              bool
	destructorDone                      bool
	cleanupChunksNeeded                 bool
	nextReleaseSerial                   uint64
	rawRelease                          rawAllocatorReleaseState
}
type AllocatorStats struct {
	OwnerBytes, LocationBytes, TotalBytes uint64
	Chunks, DirectObjects, PendingBlocks  uint32
}

func NewSmallAllocator(c SmallAllocatorConfig) (*SmallAllocator, error) {
	if c.Runtime == nil {
		return nil, ErrAllocatorOwner
	}
	if e := c.Runtime.open(); e != nil {
		return nil, e
	}
	if c.Profile.PointerBits == 0 {
		c.Profile.PointerBits = c.Runtime.config.SizeTBits
	}
	if (c.Profile.PointerBits != 32 && c.Profile.PointerBits != 64) || c.Profile.PointerBits != c.Runtime.config.SizeTBits {
		return nil, arena.ErrConfig
	}
	if e := arena.ValidateRecordType(reflect.TypeOf(smallObjectLocation{})); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(SmallAllocator{}).Size())
	if e := c.Runtime.config.Budget.Reserve(n); e != nil {
		return nil, e
	}
	var id uint64
	for {
		old := smallObjectOrigins.Load()
		if old == math.MaxUint64 {
			c.Runtime.config.Budget.Release(n)
			return nil, arena.ErrSlotLimit
		}
		if smallObjectOrigins.CompareAndSwap(old, old+1) {
			id = old + 1
			break
		}
	}
	a := &SmallAllocator{config: c, origin: id, header: n}
	a.self = a
	return a, nil
}
func (a *SmallAllocator) entryOpen() error {
	if a == nil || a.self != a {
		return ErrAllocatorOwner
	}
	if a.closed || a.destructorDone {
		return arena.ErrClosed
	}
	return a.config.Runtime.open()
}
func (a *SmallAllocator) open() error {
	if e := a.entryOpen(); e != nil {
		return e
	}
	if a.rawRelease.active {
		return ErrAllocatorPending
	}
	return nil
}
func (a *SmallAllocator) pointerBytes() uint64 {
	return uint64(a.config.Profile.PointerBits / 8)
}
func (a *SmallAllocator) slots() uint32 { return uint32(256 / a.pointerBytes()) }
func (a *SmallAllocator) bypass() bool {
	return a.config.Profile.Debug && !a.config.Profile.Windows
}
func (a *SmallAllocator) counter(n uint64) uint64 {
	if a.config.Profile.PointerBits == 32 {
		return uint64(uint32(n))
	}
	return n
}
func (a *SmallAllocator) reserveLocations(extra uint32) error {
	available := uint64(cap(a.locations) - len(a.locations))
	for i := range a.locations {
		r := &a.locations[i]
		if !r.live && !r.free && !r.retired && r.raw == (Block{}) && r.generation < math.MaxUint32 {
			available++
		}
	}
	if available >= uint64(extra) {
		return nil
	}
	missing := uint64(extra) - available
	needed := uint64(cap(a.locations)) + missing
	if needed > math.MaxUint32 {
		return arena.ErrSlotLimit
	}
	n := uint64(cap(a.locations))
	if n == 0 {
		n = 2
	}
	for n < needed {
		next := (3*n + 1) >> 1
		if next <= n || next > math.MaxUint32 {
			return arena.ErrSlotLimit
		}
		n = next
	}
	stride := uint64(reflect.TypeOf(smallObjectLocation{}).Size())
	if n > uint64(^uint(0)>>1)/stride {
		return arena.ErrBounds
	}
	bytes := n * stride
	if e := a.config.Runtime.config.Budget.Reserve(bytes); e != nil {
		return e
	}
	next := make([]smallObjectLocation, len(a.locations), int(n))
	copy(next, a.locations)
	a.config.Runtime.config.Budget.Release(uint64(cap(a.locations)) * stride)
	a.locations = next
	return nil
}
func (a *SmallAllocator) installLocation(r smallObjectLocation) uint32 {
	for i := range a.locations {
		old := &a.locations[i]
		if !old.live && !old.free && !old.retired && old.raw == (Block{}) && old.generation < math.MaxUint32 {
			r.generation = old.generation + 1
			*old = r
			return uint32(i + 1)
		}
	}
	r.generation = 1
	a.locations = append(a.locations, r)
	return uint32(len(a.locations))
}
func (a *SmallAllocator) location(id uint32) (*smallObjectLocation, error) {
	if id == 0 || uint64(id) > uint64(len(a.locations)) {
		return nil, ErrAllocatorOwner
	}
	return &a.locations[id-1], nil
}
func (a *SmallAllocator) object(r ObjectRef) (*smallObjectLocation, error) {
	if r.owner != a.origin {
		return nil, ErrAllocatorOwner
	}
	loc, e := a.location(r.slot)
	if e != nil {
		return nil, e
	}
	if !loc.live || loc.generation != r.generation || (loc.kind != smallObjectPooled && loc.kind != smallObjectDirect) {
		return nil, ErrAllocatorOwner
	}
	return loc, nil
}
func (a *SmallAllocator) ref(id uint32) ObjectRef {
	r := a.locations[id-1]
	return ObjectRef{a.origin, r.address, id, r.generation}
}
func (a *SmallAllocator) nextAddress() (uint64, error) {
	if a.sequence >= math.MaxUint64/8192 {
		return 0, arena.ErrSlotLimit
	}
	a.sequence++
	return a.sequence * 8192, nil
}
func smallObjectReadWord(rt *Runtime, h Block, offset, width uint64) (uint64, error) {
	var word uint64
	for i := uint64(0); i < width; i++ {
		v, e := rt.Read(h, offset+i)
		if e != nil {
			return 0, e
		}
		word |= uint64(v) << (8 * i)
	}
	return word, nil
}
func smallObjectWriteWord(rt *Runtime, h Block, offset, width, v uint64) error {
	for i := uint64(0); i < width; i++ {
		if e := rt.Write(h, offset+i, byte(v>>(8*i))); e != nil {
			return e
		}
	}
	return nil
}
func (a *SmallAllocator) rememberPending(h Block) {
	if h == (Block{}) {
		return
	}
	a.pending = h
	_, e := a.config.Runtime.Size(h)
	a.pendingPartial = e != nil
}
func (a *SmallAllocator) Allocate(n uint64) (ObjectRef, error) {
	return a.allocateWithOutcome(n, nil, nil)
}
func (a *SmallAllocator) AllocateWithOutcome(n uint64, outcome *SmallAllocatorAllocationOutcome) (ObjectRef, error) {
	return a.allocateWithOutcome(n, outcome, nil)
}

// AllocateWithOutcomeStarted reports only the actual guarded outcome publication.
// Rejected and source-zero calls preserve the supplied prior outcome.
func (a *SmallAllocator) AllocateWithOutcomeStarted(n uint64, outcome *SmallAllocatorAllocationOutcome) (ObjectRef, bool, error) {
	var started bool
	object, cause := a.allocateWithOutcome(n, outcome, &started)
	return object, started, cause
}
func (a *SmallAllocator) allocateWithOutcome(n uint64, outcome *SmallAllocatorAllocationOutcome, started *bool) (ObjectRef, error) {
	if e := a.entryOpen(); e != nil {
		return ObjectRef{}, e
	}
	// Source size_t argument conversion precedes the body zero shortcut.
	n = a.counter(n)
	if n == 0 {
		return ObjectRef{}, nil
	}
	if a.rawRelease.active {
		return ObjectRef{}, ErrAllocatorPending
	}
	if a.hasPending() || a.cleanupMode != 0 || a.cleanupChunksNeeded {
		return ObjectRef{}, ErrAllocatorPending
	}
	if outcome != nil {
		frame := &a.allocationInvocation
		if frame.executing || frame.invocation == ^uint64(0) {
			return ObjectRef{}, ErrAllocatorPending
		}
		frame.invocation++
		frame.executing = true
		*outcome = SmallAllocatorAllocationOutcome{owner: a, frame: frame, invocation: frame.invocation}
		if started != nil {
			*started = true
		}
		defer func() { frame.executing = false }()
	}
	if !a.bypass() {
		a.allocSize = a.counter(a.allocSize + n)
	}
	width := a.pointerBytes()
	if a.bypass() || n >= 256-width {
		if e := a.reserveLocations(1); e != nil {
			return ObjectRef{}, e
		}
		address, e := a.nextAddress()
		if e != nil {
			return ObjectRef{}, e
		}
		h, e := a.config.Runtime.Allocate(n)
		if e != nil {
			a.rememberPendingWithOutcome(h, outcome)
			return ObjectRef{}, e
		}
		id := a.installLocation(smallObjectLocation{raw: h, address: address, requested: n, kind: smallObjectDirect, live: true})
		return a.recordAllocationObject(outcome, a.ref(id), true), nil
	}
	slot := uint32((n + width - 1) / width)
	if e := a.sourceAssert(slot < a.slots()); e != nil {
		return ObjectRef{}, e
	}
	if e := a.sourceAssert(slot > 0); e != nil {
		return ObjectRef{}, e
	}
	if slot >= a.slots() {
		return ObjectRef{}, arena.ErrBounds
	}
	if a.free[slot] != 0 {
		id := a.free[slot]
		loc, e := a.location(id)
		if e != nil {
			return ObjectRef{}, e
		}
		next, e := smallObjectReadWord(a.config.Runtime, loc.raw, uint64(loc.offset), width)
		if e != nil {
			return ObjectRef{}, e
		}
		if next > math.MaxUint32 {
			return ObjectRef{}, ErrAllocatorOwner
		}
		a.free[slot] = uint32(next)
		if loc.generation == math.MaxUint32 {
			loc.retired = true
			loc.free = false
			return ObjectRef{}, arena.ErrSlotLimit
		}
		loc.generation++
		loc.release = allocatorReleaseState{}
		loc.free = false
		loc.live = true
		loc.requested = n
		loc.extent = slot * uint32(width)
		return a.recordAllocationObject(outcome, a.ref(id), false), nil
	}
	if e := a.reserveLocations(1); e != nil {
		return ObjectRef{}, e
	}
	size := uint64(slot) * width
	if e := a.sourceAssert(size >= n); e != nil {
		return ObjectRef{}, e
	}
	if a.chunks[slot] != 0 {
		chunk, e := a.location(a.chunks[slot])
		if e != nil {
			return ObjectRef{}, e
		}
		curr, e := smallObjectReadWord(a.config.Runtime, chunk.raw, width, width)
		if e != nil {
			return ObjectRef{}, e
		}
		if curr > 8192 {
			return ObjectRef{}, arena.ErrBounds
		}
		if curr+size < 8192 {
			if e = smallObjectWriteWord(a.config.Runtime, chunk.raw, width, width, curr+size); e != nil {
				return ObjectRef{}, e
			}
			id := a.installLocation(smallObjectLocation{raw: chunk.raw, address: chunk.address + curr, requested: n, offset: uint32(curr), extent: uint32(size), chunk: a.chunks[slot], kind: smallObjectPooled, live: true})
			return a.recordAllocationObject(outcome, a.ref(id), false), nil
		}
	}
	if e := a.reserveLocations(2); e != nil {
		return ObjectRef{}, e
	}
	address, e := a.nextAddress()
	if e != nil {
		return ObjectRef{}, e
	}
	h, e := a.config.Runtime.Allocate(8192)
	if e != nil {
		a.rememberPendingWithOutcome(h, outcome)
		return ObjectRef{}, e
	}
	if a.config.Profile.Debug && a.config.ChunkTrace != nil {
		a.config.ChunkTrace.AllocatedChunk(address)
	}
	// ctor sets curr first; source next and chain publication follow.
	if e = smallObjectWriteWord(a.config.Runtime, h, width, width, 2*width); e != nil {
		a.rememberPendingWithOutcome(h, outcome)
		return ObjectRef{}, e
	}
	if e = smallObjectWriteWord(a.config.Runtime, h, 0, width, uint64(a.chunks[slot])); e != nil {
		a.rememberPendingWithOutcome(h, outcome)
		return ObjectRef{}, e
	}
	chunkID := a.installLocation(smallObjectLocation{raw: h, address: address, kind: smallObjectChunk, live: true})
	a.chunks[slot] = chunkID
	if e = smallObjectWriteWord(a.config.Runtime, h, width, width, 2*width+size); e != nil {
		return ObjectRef{}, e
	}
	id := a.installLocation(smallObjectLocation{raw: h, address: address + 2*width, requested: n, offset: uint32(2 * width), extent: uint32(size), chunk: chunkID, kind: smallObjectPooled, live: true})
	return a.recordAllocationObject(outcome, a.ref(id), true), nil
}
func (a *SmallAllocator) Deallocate(n uint64, r ObjectRef) error {
	_, e := a.DeallocateTracked(n, r)
	return e
}
func (a *SmallAllocator) RawLocation(r ObjectRef) (Block, uint64, error) {
	if e := a.open(); e != nil {
		return Block{}, 0, e
	}
	loc, e := a.object(r)
	if e != nil {
		return Block{}, 0, e
	}
	return loc.raw, uint64(loc.offset), nil
}
func (a *SmallAllocator) Read(r ObjectRef, i uint64) (byte, error) {
	if e := a.open(); e != nil {
		return 0, e
	}
	loc, e := a.object(r)
	if e != nil {
		return 0, e
	}
	if i >= loc.requested {
		return 0, arena.ErrBounds
	}
	return a.config.Runtime.Read(loc.raw, uint64(loc.offset)+i)
}
func (a *SmallAllocator) Write(r ObjectRef, i uint64, v byte) error {
	if e := a.open(); e != nil {
		return e
	}
	loc, e := a.object(r)
	if e != nil {
		return e
	}
	if i >= loc.requested {
		return arena.ErrBounds
	}
	return a.config.Runtime.Write(loc.raw, uint64(loc.offset)+i, v)
}
func (a *SmallAllocator) AllocationSize() (uint64, error) {
	if e := a.entryOpen(); e != nil {
		return 0, e
	}
	return a.allocSize, nil
}
func (a *SmallAllocator) freeMeasure(waste bool) (uint64, error) {
	if e := a.entryOpen(); e != nil {
		return 0, e
	}
	var result uint64
	for slot := uint32(0); slot < a.slots(); slot++ {
		id := a.free[slot]
		steps := 0
		for id != 0 {
			if steps >= len(a.locations) {
				return result, ErrAllocatorNonterminating
			}
			steps++
			loc, e := a.location(id)
			if e != nil {
				return result, e
			}
			if waste {
				result = a.counter(result + uint64(slot)*a.pointerBytes())
			} else {
				result = a.counter(result + 1)
			}
			next, e := smallObjectReadWord(a.config.Runtime, loc.raw, uint64(loc.offset), a.pointerBytes())
			if e != nil {
				return result, e
			}
			if next > math.MaxUint32 {
				return result, ErrAllocatorOwner
			}
			id = uint32(next)
		}
	}
	return result, nil
}
func (a *SmallAllocator) WastedSize() (uint64, error)     { return a.freeMeasure(true) }
func (a *SmallAllocator) NumFreeObjects() (uint64, error) { return a.freeMeasure(false) }
func (a *SmallAllocator) invalidateChunk(id uint32) {
	for i := range a.locations {
		r := &a.locations[i]
		if uint32(i+1) == id || r.kind == smallObjectPooled && r.chunk == id {
			r.live = false
			r.free = false
			r.raw = Block{}
			if r.generation == math.MaxUint32 {
				r.retired = true
			}
		}
	}
}

// The cold cursor retains real remaining roots if a raw deallocation transport
// reports an error after physical free. Retrying cleanup is a Go continuation.
func (a *SmallAllocator) freeChunks(mode uint8) error {
	if a.cleanupChunksNeeded {
		return ErrAllocatorPending
	}
	if a.cleanupMode != 0 && a.cleanupMode != mode {
		return ErrAllocatorPending
	}
	if a.cleanupMode == 0 {
		a.cleanupMode = mode
		a.cleanupClass = 0
		a.cleanupCurrent = a.chunks[0]
	}
	for a.cleanupClass < a.slots() {
		if a.cleanupCurrent == 0 {
			a.chunks[a.cleanupClass] = 0
			a.free[a.cleanupClass] = 0
			a.cleanupClass++
			if a.cleanupClass < a.slots() {
				a.cleanupCurrent = a.chunks[a.cleanupClass]
			}
			continue
		}
		id := a.cleanupCurrent
		loc, e := a.location(id)
		if e != nil {
			return e
		}
		next, e := smallObjectReadWord(a.config.Runtime, loc.raw, 0, a.pointerBytes())
		if e != nil {
			return e
		}
		if next > math.MaxUint32 {
			return ErrAllocatorOwner
		}
		h := loc.raw
		e = a.config.Runtime.Deallocate(&h)
		if h == (Block{}) {
			if a.config.Profile.Debug && a.config.ChunkTrace != nil {
				line := 60 // reset: dealloc(c)
				if mode == 2 {
					line = 44 // destructor: dealloc(c)
				}
				a.config.ChunkTrace.DeallocatedChunk(loc.address, line)
			}
			a.invalidateChunk(id)
			a.cleanupCurrent = uint32(next)
		}
		if e != nil {
			return e
		}
	}
	a.cleanupMode = 0
	return nil
}
func (a *SmallAllocator) Reset() error {
	if e := a.open(); e != nil {
		return e
	}
	if e := a.freeChunks(1); e != nil {
		return e
	}
	a.allocSize = 0
	return nil
}
func (a *SmallAllocator) hasPending() bool {
	return a.pending != (Block{})
}
func (a *SmallAllocator) CleanupPending() error {
	if a == nil || a.self != a {
		return ErrAllocatorOwner
	}
	if a.pending != (Block{}) {
		var e error
		if a.pendingPartial {
			e = a.config.Runtime.CleanupPartial(&a.pending)
		} else {
			e = a.config.Runtime.Deallocate(&a.pending)
		}
		if a.pending == (Block{}) {
			a.pendingPartial = false
		}
		if e != nil {
			return e
		}
	}
	return nil
}
func (a *SmallAllocator) Close() error {
	if a == nil || a.self != a {
		return ErrAllocatorOwner
	}
	if a.closed {
		return nil
	}
	if e := a.config.Runtime.open(); e != nil {
		return e
	}
	if a.rawRelease.active {
		return ErrAllocatorPending
	}
	var e error
	if !a.destructorDone {
		if e = a.freeChunks(2); e != nil {
			return e
		}
		e = a.diagnostic()
		// Completed destructor actions, including any emitted diagnostic prefix,
		// are never replayed by owner-finalization retries.
		a.destructorDone = true
	}
	if a.hasPending() {
		if e != nil {
			return e
		}
		return ErrAllocatorPending
	}
	a.config.Runtime.config.Budget.Release(a.header + uint64(cap(a.locations))*uint64(reflect.TypeOf(smallObjectLocation{}).Size()))
	a.locations = nil
	a.header = 0
	a.closed = true
	return e
}
func (a *SmallAllocator) Stats() AllocatorStats {
	if a == nil || a.self != a {
		return AllocatorStats{}
	}
	s := AllocatorStats{OwnerBytes: a.header, LocationBytes: uint64(cap(a.locations)) * uint64(reflect.TypeOf(smallObjectLocation{}).Size())}
	s.TotalBytes = s.OwnerBytes + s.LocationBytes
	for _, r := range a.locations {
		if r.live && r.kind == smallObjectChunk {
			s.Chunks++
		}
		if r.live && r.kind == smallObjectDirect {
			s.DirectObjects++
		}
	}
	if a.pending != (Block{}) {
		s.PendingBlocks = 1
	}
	return s
}
func (a *SmallAllocator) PlacementDelete(ObjectRef) error {
	return ErrAllocatorUnreachable
}
func (a *SmallAllocator) PlacementDeleteArray(ObjectRef) error {
	return ErrAllocatorUnreachable
}

// CleanupChunks is an explicit Go continuation after an interrupted source
// Consolidate. Source heads are left untouched at failure; retained cold
// locators keep every actual owning root accessible without raw traversal.
func (a *SmallAllocator) CleanupChunks() error {
	if e := a.open(); e != nil {
		return e
	}
	if !a.cleanupChunksNeeded {
		return nil
	}
	for i := range a.locations {
		loc := &a.locations[i]
		if loc.kind != smallObjectChunk || !loc.live {
			continue
		}
		h := loc.raw
		e := a.config.Runtime.Deallocate(&h)
		if h == (Block{}) {
			if a.config.Profile.Debug && a.config.ChunkTrace != nil {
				a.config.ChunkTrace.DeallocatedChunk(loc.address, 221)
			}
			a.invalidateChunk(uint32(i + 1))
		}
		if e != nil {
			return e
		}
	}
	a.chunks = [64]uint32{}
	a.free = [64]uint32{}
	a.cleanupChunksNeeded = false
	return nil
}
