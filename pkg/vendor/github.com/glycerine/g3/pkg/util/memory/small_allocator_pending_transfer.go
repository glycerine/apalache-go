package memory

// Source: src/util/small_object_allocator.cpp:allocate/deallocate.

// Embedded in the charged caller frame. This owns only a failed raw allocation
// that was never joined to an allocator object or chunk.
type SmallAllocatorPendingAllocation struct {
	self       *SmallAllocatorPendingAllocation
	owner      *SmallAllocator
	invocation uint64
	block      Block
	partial    bool
}

func (p *SmallAllocatorPendingAllocation) Block() Block {
	if p == nil {
		return Block{}
	}
	return p.block
}
func (a *SmallAllocator) TransferAllocationPending(o *SmallAllocatorAllocationOutcome, p *SmallAllocatorPendingAllocation) error {
	if a == nil || a.self != a || p == nil || !a.MatchesAllocationOutcome(o) || o.Pending == (Block{}) || o.Pending != a.pending || o.Object != (ObjectRef{}) || !o.FreshRaw {
		return ErrAllocatorOwner
	}
	if a.allocationInvocation.executing || a.config.Runtime.FailureTransportActive() {
		return ErrAllocatorPending
	}
	if *p != (SmallAllocatorPendingAllocation{}) {
		return ErrAllocatorOwner
	}
	*p = SmallAllocatorPendingAllocation{self: p, owner: a, invocation: o.invocation, block: a.pending, partial: a.pendingPartial}
	a.pending = Block{}
	a.pendingPartial = false
	return nil
}
func (a *SmallAllocator) CleanupTransferredAllocation(p *SmallAllocatorPendingAllocation) error {
	if a == nil || a.self != a || p == nil || p.self != p || p.owner != a || p.invocation == 0 {
		return ErrAllocatorOwner
	}
	if a.allocationInvocation.executing || a.config.Runtime.FailureTransportActive() {
		return ErrAllocatorPending
	}
	if e := a.config.Runtime.open(); e != nil {
		return e
	}
	if p.block == (Block{}) {
		return nil
	}
	if p.partial {
		return a.config.Runtime.CleanupPartial(&p.block)
	}
	return a.config.Runtime.Deallocate(&p.block)
}

// SourceObjectOwnerEnded reports completion of the literal allocator destructor.
// It does not identify or discover allocations retained by an external caller.
func (a *SmallAllocator) SourceObjectOwnerEnded() (bool, error) {
	if a == nil || a.self != a {
		return false, ErrAllocatorOwner
	}
	return a.closed, nil
}

// CleanupSourceRetainedDirect handles an exact caller-owned direct object when
// source destruction completed but a different pending root retains the map.
// The object token and raw binding are validated before physical release.
func (a *SmallAllocator) CleanupSourceRetainedDirect(r ObjectRef, raw *Block) (bool, error) {
	if a == nil || a.self != a || raw == nil {
		return false, ErrAllocatorOwner
	}
	if !a.destructorDone || a.closed {
		return false, nil
	}
	if a.allocationInvocation.executing || a.config.Runtime.FailureTransportActive() {
		return true, ErrAllocatorPending
	}
	loc, e := a.object(r)
	if e != nil {
		return true, e
	}
	if loc.kind != smallObjectDirect || loc.raw != *raw || loc.release.active {
		return true, ErrAllocatorOwner
	}
	e = a.config.Runtime.Deallocate(raw)
	if *raw == (Block{}) {
		loc.live = false
		loc.raw = Block{}
		loc.retired = true
	}
	return true, e
}
