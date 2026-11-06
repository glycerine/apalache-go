package memory

import "errors"

var ErrProcessLifecyclePending = errors.New("g3: source memory lifecycle stage is pending")
var ErrProcessLifecycleOrder = errors.New("g3: source memory lifecycle stage out of order")

// ProcessAccounting is the shared source memory_manager.cpp counter and limit
// state for a group of Runtime owners. The caller confines the process and its
// runtimes to one goroutine, as with the individual Runtime owners. Each runtime
// keeps its own allocation lane, including the source TLS synchronization
// threshold; only the global fields are shared. The first successful runtime
// fixes the source process's size_t width and thread-local allocator branch
// for later runtime owners.
type ProcessAccounting struct {
	self                                      *ProcessAccounting
	accounting                                utilMemoryAccounting
	fatalPolicy                               FatalPolicy
	sizeTBits                                 uint8
	threadLocal                               bool
	initialized, fullyInitialized, finalizing bool
	initializing                              bool
}

func NewProcessAccounting() *ProcessAccounting {
	p := &ProcessAccounting{}
	p.self = p
	return p
}

// Valid reports whether this is the original, initialized process owner.
func (p *ProcessAccounting) Valid() bool { return p != nil && p.self == p }

// AllocationSize is the process-wide source memory::get_allocation_size value.
// It excludes unsynchronized thread-local lane deltas, as the source getter does.
func (p *ProcessAccounting) AllocationSize() uint64 {
	if !p.Valid() {
		return 0
	}
	return p.accounting.allocationSize()
}

// MaximumUsed and AllocationCount are the other process-wide getters consumed
// by src/util/statistics.cpp's get_memory_statistics.
func (p *ProcessAccounting) MaximumUsed() uint64 {
	if !p.Valid() {
		return 0
	}
	return p.accounting.maximumUsed()
}

func (p *ProcessAccounting) AllocationCount() uint64 {
	if !p.Valid() {
		return 0
	}
	return p.accounting.allocationCount()
}

// OutOfMemory is memory::is_out_of_memory's sticky process-wide flag.
func (p *ProcessAccounting) OutOfMemory() bool {
	return p.Valid() && p.accounting.outOfMemory
}

// AboveWatermark is memory::above_high_watermark. A zero watermark disables
// the predicate; the comparison uses only the synchronized global byte count.
func (p *ProcessAccounting) AboveWatermark() bool {
	return p.Valid() && p.accounting.aboveWatermark()
}

// InitializationStarted reports the source g_memory_initialized guard. It is
// retained across generated child construction and cleared by finalization.
func (p *ProcessAccounting) InitializationStarted() bool {
	return p.Valid() && p.initialized
}

// Finalizing reports the source g_finalizing guard used by memory's debug
// deallocation trace while generated process children are being destroyed.
func (p *ProcessAccounting) Finalizing() bool {
	return p.Valid() && p.finalizing
}

// BeginFinalize enters memory::finalize's guarded generated-child interval.
// The caller runs mem_finalize's concrete children before CompleteFinalize.
func (p *ProcessAccounting) BeginFinalize() (bool, error) {
	if !p.Valid() {
		return false, ErrRuntimeOwner
	}
	if p.initializing || p.finalizing {
		return false, ErrProcessLifecyclePending
	}
	if !p.initialized {
		return false, nil
	}
	p.finalizing = true
	return true, nil
}

// CompleteFinalize matches the two assignments after generated mem_finalize.
// The source does not clear g_memory_fully_initialized on finalization.
func (p *ProcessAccounting) CompleteFinalize() error {
	if !p.Valid() {
		return ErrRuntimeOwner
	}
	if !p.finalizing {
		return ErrProcessLifecycleOrder
	}
	p.initialized = false
	p.finalizing = false
	return nil
}
