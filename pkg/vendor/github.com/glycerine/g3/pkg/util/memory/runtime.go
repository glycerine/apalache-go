package memory

// Source: src/util/memory_manager.cpp:33-72,124-149,274-305,326-377; src/util/memory_manager.h:49-71.

import (
	"errors"
	"math"
	"reflect"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"
)

var ErrRuntimeOwner = errors.New("g3: invalid raw memory owner or block")
var ErrRuntimeLive = errors.New("g3: live raw memory blocks")
var ErrRuntimeUninitialized = errors.New("g3: uninitialized raw memory byte")
var memoryRuntimeOrigins atomic.Uint64

type RuntimeConfig struct {
	Budget      *arena.Budget
	Process     *ProcessAccounting
	ThreadLocal bool
	Transport   ErrorTransport
	FatalPolicy FatalPolicy
	SizeTBits   uint8
}
type Block struct {
	origin           uint64
	slot, generation uint32
}

// Each entry owns one actual malloc backing, plus checked initialized-byte bits.
// Both slice headers are cold allocation metadata, never boxed hot records.
type memoryRuntimeBlock struct {
	data        []byte
	initialized []uint64
	generation  uint32
	live        bool
	release     runtimeReleaseState
}
type Runtime struct {
	self                  *Runtime
	config                RuntimeConfig
	origin, header        uint64
	accounting            utilMemoryAccounting
	lane                  utilMemoryLaneAccounting
	blocks                []memoryRuntimeBlock
	live                  uint32
	closed                bool
	nextReleaseSerial     uint64
	failureTransportDepth uint32
}
type RuntimeState struct {
	GlobalSize, LocalSize, GlobalCount, LocalCount, Peak int64
	OutOfMemory                                          bool
}
type RuntimeStats struct {
	OwnerBytes, DirectoryBytes, PhysicalBytes, InitializationBytes, TotalBytes uint64
	LiveBlocks                                                                 uint32
}

// This is the explicit owner of the real raw allocate/reallocate/deallocate
// paths, not generated memory::initialize/finalize or runtime-wide typed hooks.
// One confined lane implements the selected TLS/non-TLS source body. Owners
// and their Budget require caller confinement. Runtimes that belong to the
// same source memory process share a ProcessAccounting owner.
func NewRuntime(c RuntimeConfig) (*Runtime, error) {
	if c.Process != nil && !c.Process.Valid() {
		return nil, arena.ErrConfig
	}
	if c.Process != nil && c.Process.fatalPolicy != nil {
		bound := c.Process.fatalPolicy
		if c.FatalPolicy != nil {
			t := reflect.TypeOf(bound)
			if t != reflect.TypeOf(c.FatalPolicy) || !t.Comparable() || bound != c.FatalPolicy {
				return nil, arena.ErrConfig
			}
		}
		c.FatalPolicy = bound
	}
	if c.FatalPolicy != nil && (c.Transport != nil || c.FatalPolicy.MemoryBudget() != c.Budget) {
		return nil, arena.ErrConfig
	}
	if c.SizeTBits == 0 {
		c.SizeTBits = 64
	}
	if c.SizeTBits != 32 && c.SizeTBits != 64 {
		return nil, arena.ErrConfig
	}
	if c.Process != nil && c.Process.sizeTBits != 0 && (c.Process.sizeTBits != c.SizeTBits || c.Process.threadLocal != c.ThreadLocal) {
		return nil, arena.ErrConfig
	}
	n := uint64(reflect.TypeOf(Runtime{}).Size())
	if e := c.Budget.Reserve(n); e != nil {
		return nil, e
	}
	var id uint64
	for {
		v := memoryRuntimeOrigins.Load()
		if v == math.MaxUint64 {
			c.Budget.Release(n)
			return nil, arena.ErrSlotLimit
		}
		if memoryRuntimeOrigins.CompareAndSwap(v, v+1) {
			id = v + 1
			break
		}
	}
	m := &Runtime{config: c, origin: id, header: n}
	m.self = m
	if c.Process != nil {
		c.Process.sizeTBits = c.SizeTBits
		c.Process.threadLocal = c.ThreadLocal
		if c.Process.fatalPolicy == nil {
			c.Process.fatalPolicy = c.FatalPolicy
		}
	}
	return m, nil
}
func (m *Runtime) accountingOwner() *utilMemoryAccounting {
	if m.config.Process != nil {
		return &m.config.Process.accounting
	}
	return &m.accounting
}

// ProcessOwner reports the source process accounting shared by this runtime.
// A nil result means this runtime uses its standalone accounting fields.
func (m *Runtime) ProcessOwner() *ProcessAccounting {
	if m == nil || m.self != m || m.closed {
		return nil
	}
	return m.config.Process
}
func (m *Runtime) open() error {
	if m == nil || m.self != m {
		return ErrRuntimeOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	return nil
}
func (m *Runtime) block(h Block) (*memoryRuntimeBlock, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	if h.origin != m.origin || h.slot == 0 || uint64(h.slot) > uint64(len(m.blocks)) {
		return nil, ErrRuntimeOwner
	}
	b := &m.blocks[h.slot-1]
	if !b.live || b.generation != h.generation {
		return nil, ErrRuntimeOwner
	}
	return b, nil
}
func (m *Runtime) extent(n uint64) (uint64, error) {
	if m.config.SizeTBits == 32 {
		// Convert the actual size_t argument before adding the SIZE_T header.
		return uint64(uint32(n) + 4), nil
	}
	return n + 8, nil
}
func (m *Runtime) transport(r memoryAccountingResult) error {
	switch r {
	case memoryAccountingOK:
		return nil
	case memoryAccountingOOM:
		return m.emitFailure(FailureMemout)
	case memoryAccountingCountLimit:
		return m.emitFailure(FailureAllocExceeded)
	default:
		return arena.ErrBounds
	}
}
func (m *Runtime) physicalFailure() error {
	m.accountingOwner().outOfMemory = true
	return m.emitFailure(FailureMemout)
}
func (m *Runtime) slot() (uint32, error) {
	for i := range m.blocks {
		b := &m.blocks[i]
		if !b.live && b.generation != math.MaxUint32 {
			return uint32(i + 1), nil
		}
	}
	if uint64(len(m.blocks)) >= math.MaxUint32 {
		return 0, arena.ErrSlotLimit
	}
	if len(m.blocks) == cap(m.blocks) {
		n := cap(m.blocks) * 2
		if n == 0 {
			n = 2
		}
		stride := uint64(reflect.TypeOf(memoryRuntimeBlock{}).Size())
		if n < cap(m.blocks) || uint64(n) > uint64(^uint(0)>>1)/stride {
			return 0, arena.ErrBounds
		}
		if e := m.config.Budget.Reserve(uint64(n) * stride); e != nil {
			return 0, m.physicalFailure()
		}
		next := make([]memoryRuntimeBlock, len(m.blocks), n)
		copy(next, m.blocks)
		m.config.Budget.Release(uint64(cap(m.blocks)) * stride)
		m.blocks = next
	}
	m.blocks = append(m.blocks, memoryRuntimeBlock{generation: 1})
	return uint32(len(m.blocks)), nil
}
func (m *Runtime) storage(n uint64) ([]byte, []uint64, error) {
	// Bound actual Go allocations separately from source SIZE_T arithmetic.
	// Too-short wrapped extents remain owned until the actual header-write guard.
	if n > uint64(^uint(0)>>1) {
		return nil, nil, arena.ErrBounds
	}
	words := (n + 63) / 64
	if words > math.MaxUint64/8 || n > math.MaxUint64-words*8 {
		return nil, nil, arena.ErrBounds
	}
	if e := m.config.Budget.Reserve(n + words*8); e != nil {
		return nil, nil, m.physicalFailure()
	}
	return make([]byte, int(n)), make([]uint64, int(words)), nil
}
func (m *Runtime) headerWrite(b *memoryRuntimeBlock, n uint64) error {
	if len(b.data) < int(m.config.SizeTBits/8) {
		return arena.ErrBounds
	}
	for i := uint8(0); i < m.config.SizeTBits/8; i++ {
		b.data[i] = byte(n >> (8 * i))
		b.initialized[0] |= uint64(1) << i
	}
	return nil
}
func (m *Runtime) headerRead(b *memoryRuntimeBlock) (uint64, error) {
	if len(b.data) < int(m.config.SizeTBits/8) {
		return 0, arena.ErrBounds
	}
	var n uint64
	for i := uint8(0); i < m.config.SizeTBits/8; i++ {
		n |= uint64(b.data[i]) << (8 * i)
	}
	return n, nil
}
func (m *Runtime) Allocate(n uint64) (Block, error) {
	if e := m.open(); e != nil {
		return Block{}, e
	}
	extent, e := m.extent(n)
	if e != nil {
		return Block{}, e
	}
	if !m.config.ThreadLocal {
		if e = m.transport(m.accountingOwner().accountExtentAllocation(&m.lane, extent, m.config.SizeTBits, false)); e != nil {
			return Block{}, e
		}
	}
	slot, e := m.slot()
	if e != nil {
		return Block{}, e
	}
	data, init, e := m.storage(extent)
	if e != nil {
		return Block{}, e
	}
	b := &m.blocks[slot-1]
	b.data = data
	b.initialized = init
	b.live = true
	b.release = runtimeReleaseState{}
	m.live++
	h := Block{m.origin, slot, b.generation}
	if e = m.headerWrite(b, extent); e != nil {
		return h, e
	}
	if m.config.ThreadLocal {
		e = m.transport(m.accountingOwner().accountExtentAllocation(&m.lane, extent, m.config.SizeTBits, true))
	}
	// TLS limit checks follow real allocation/header/counter publication. Return
	// the owned completed block even with an error; caller must Deallocate it.
	return h, e
}
func (m *Runtime) Reallocate(h *Block, n uint64) error {
	if h == nil {
		return ErrRuntimeOwner
	}
	b, e := m.block(*h)
	if e != nil {
		return e
	}
	// Exhausted checked pointer generations cannot publish another realloc
	// binding. This explicit token-domain guard precedes source actions.
	if b.generation == math.MaxUint32 {
		return arena.ErrSlotLimit
	}
	old, e := m.headerRead(b)
	if e != nil {
		return e
	}
	extent, e := m.extent(n)
	if e != nil {
		return e
	}
	delta := extent - old
	if m.config.SizeTBits == 32 {
		delta = uint64(uint32(extent) - uint32(old))
	}
	if e = m.transport(m.accountingOwner().accountExtentAllocation(&m.lane, delta, m.config.SizeTBits, m.config.ThreadLocal)); e != nil {
		return e
	}
	data, init, e := m.storage(extent)
	if e != nil {
		return e
	}
	copy(data, b.data)
	copy(init, b.initialized)
	// Byte-validity of newly grown storage must not copy unused high bits.
	if extent > old && old%64 != 0 {
		init[old/64] &= (uint64(1) << (old % 64)) - 1
	}
	m.config.Budget.Release(uint64(len(b.data)) + uint64(len(b.initialized))*8)
	b.data = data
	b.initialized = init
	// The checked descriptor tracks malloc ownership, not a raw pointer address.
	// A successful realloc invalidates old pointer tokens, including equal sizes.
	b.generation++
	b.release = runtimeReleaseState{}
	h.generation = b.generation
	return m.headerWrite(b, extent)
}
func (m *Runtime) Deallocate(h *Block) error {
	_, e := m.deallocateTracked(h)
	return e
}
func (m *Runtime) Size(h Block) (uint64, error) {
	b, e := m.block(h)
	if e != nil {
		return 0, e
	}
	n, e := m.headerRead(b)
	if e != nil {
		return 0, e
	}
	return n - uint64(m.config.SizeTBits/8), nil
}
func (m *Runtime) Read(h Block, i uint64) (byte, error) {
	b, e := m.block(h)
	if e != nil {
		return 0, e
	}
	n, e := m.headerRead(b)
	if e != nil {
		return 0, e
	}
	offset := uint64(m.config.SizeTBits / 8)
	if i >= n-offset {
		return 0, arena.ErrBounds
	}
	i += offset
	if b.initialized[i/64]&(uint64(1)<<(i%64)) == 0 {
		return 0, ErrRuntimeUninitialized
	}
	return b.data[i], nil
}
func (m *Runtime) Write(h Block, i uint64, v byte) error {
	b, e := m.block(h)
	if e != nil {
		return e
	}
	n, e := m.headerRead(b)
	if e != nil {
		return e
	}
	offset := uint64(m.config.SizeTBits / 8)
	if i >= n-offset {
		return arena.ErrBounds
	}
	i += offset
	b.data[i] = v
	b.initialized[i/64] |= uint64(1) << (i % 64)
	return nil
}
func (m *Runtime) SetMaxSize(n uint64) error {
	if e := m.open(); e != nil {
		return e
	}
	if m.config.SizeTBits == 32 {
		n = uint64(uint32(n))
	}
	m.accountingOwner().maxSize = int64(n)
	return nil
}

// SetMaxSizeMegabytes preserves util.h's saturating megabytes_to_bytes
// conversion before memory_manager.cpp stores the resulting size_t limit.
func (m *Runtime) SetMaxSizeMegabytes(mb uint32) error {
	if e := m.open(); e != nil {
		return e
	}
	limit := uint64(^uint64(0))
	if m.config.SizeTBits == 32 {
		limit = uint64(^uint32(0))
	}
	return m.SetMaxSize(megabytesToBytesLimit(mb, limit))
}
func (m *Runtime) SetMaxCount(n uint64) error {
	if e := m.open(); e != nil {
		return e
	}
	if m.config.SizeTBits == 32 {
		n = uint64(uint32(n))
	}
	m.accountingOwner().maxCount = int64(n)
	return nil
}
func (m *Runtime) SetWatermark(n uint64) error {
	if e := m.open(); e != nil {
		return e
	}
	if m.config.SizeTBits == 32 {
		n = uint64(uint32(n))
	}
	m.accountingOwner().watermark = int64(n)
	return nil
}
func (m *Runtime) AboveWatermark() (bool, error) {
	if e := m.open(); e != nil {
		return false, e
	}
	return m.accountingOwner().aboveWatermark(), nil
}
func (m *Runtime) State() RuntimeState {
	if m == nil || m.self != m || m.closed {
		return RuntimeState{}
	}
	a := m.accountingOwner()
	l := &m.lane
	return RuntimeState{a.size, l.size, a.count, l.count, a.peak, a.outOfMemory}
}
func (m *Runtime) Stats() RuntimeStats {
	if m == nil || m.self != m || m.closed {
		return RuntimeStats{}
	}
	s := RuntimeStats{OwnerBytes: m.header, DirectoryBytes: uint64(cap(m.blocks)) * uint64(reflect.TypeOf(memoryRuntimeBlock{}).Size()), LiveBlocks: m.live}
	for i := range m.blocks {
		b := &m.blocks[i]
		if b.live {
			s.PhysicalBytes += uint64(len(b.data))
			s.InitializationBytes += uint64(len(b.initialized)) * 8
		}
	}
	s.TotalBytes = s.OwnerBytes + s.DirectoryBytes + s.PhysicalBytes + s.InitializationBytes
	return s
}
func (m *Runtime) Close() error {
	if m == nil || m.self != m {
		return ErrRuntimeOwner
	}
	if m.closed {
		return nil
	}
	if m.live != 0 {
		return ErrRuntimeLive
	}
	m.config.Budget.Release(m.header + uint64(cap(m.blocks))*uint64(reflect.TypeOf(memoryRuntimeBlock{}).Size()))
	m.blocks = nil
	m.header = 0
	m.closed = true
	return nil
}

func (m *Runtime) AllocationSize() (uint64, error) {
	if e := m.open(); e != nil {
		return 0, e
	}
	return m.accountingOwner().allocationSize(), nil
}
func (m *Runtime) MaximumUsed() (uint64, error) {
	if e := m.open(); e != nil {
		return 0, e
	}
	return m.accountingOwner().maximumUsed(), nil
}
func (m *Runtime) AllocationCount() (uint64, error) {
	if e := m.open(); e != nil {
		return 0, e
	}
	return m.accountingOwner().allocationCount(), nil
}

// CleanupPartial releases only a physical block whose source SIZE_T header
// cannot exist. It performs no fabricated source counter subtraction.
func (m *Runtime) CleanupPartial(h *Block) error {
	if h == nil {
		return ErrRuntimeOwner
	}
	b, e := m.block(*h)
	if e != nil {
		return e
	}
	if len(b.data) >= int(m.config.SizeTBits/8) {
		return ErrRuntimeOwner
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
	return nil
}

// Reached source g_memory_max_alloc_count read, after the first insertion.
func (m *Runtime) ConfiguredMaxAllocationCount() (int64, error) {
	if e := m.open(); e != nil {
		return 0, e
	}
	return m.accountingOwner().maxCount, nil
}
