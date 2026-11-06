package numeral

// Source: src/util/mpf.h:241-290; src/util/mpf.cpp:1821-1847.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// Cold acquisition metadata. The sole numeral payload is the 16-byte Runtime
// wrapper, with twelve meaningful codec bytes and four unread padding bytes.
// Preparing this actual Go receipt header is an explicit adapter reservation.
type arbitraryFloatPowerWrapper struct {
	self                                                             *arbitraryFloatPowerWrapper
	manager                                                          *ArbitraryFloatManager
	Next                                                             *arbitraryFloatPowerWrapper
	Block, original                                                  umemory.Block
	Roots                                                            NumericAllocationRoots
	Transferred                                                      umemory.SmallAllocatorPendingAllocation
	originalCell                                                     SourceCellBacking
	cellRetired                                                      bool
	id                                                               uint32
	invocation, header                                               uint64
	executing, published, Initialized, SourceDelDone, sourceFreeDone bool
	memoryTraceReleased                                              bool
	bodyCause, sourceCause, ownCause                                 error
}

func (w *arbitraryFloatPowerWrapper) Valid() bool {
	return w != nil && w.self == w && w.manager != nil && w.manager.self == w.manager && !w.manager.closed && w.id != 0
}
func (w *arbitraryFloatPowerWrapper) Capture() (Int, error) {
	if !w.Valid() || w.Block == (umemory.Block{}) {
		return Int{}, umemory.ErrRuntimeOwner
	}
	r := w.manager.memory
	val, e := r.ReadUint32(w.Block, 0)
	if e != nil {
		return Int{}, e
	}
	lo, e := r.ReadUint32(w.Block, 4)
	if e != nil {
		return Int{}, e
	}
	hi, e := r.ReadUint32(w.Block, 8)
	if e != nil {
		return Int{}, e
	}
	return Int{Val: int32(val), Cell: NumericCellRef(uint64(lo) | uint64(hi)<<32)}, nil
}
func (w *arbitraryFloatPowerWrapper) receiver() *integerRuntimeValueReceiver {
	return &integerRuntimeValueReceiver{runtime: w.manager.memory, block: w.Block}
}
func (m *ArbitraryFloatManager) powerWrapperByID(id uint32) *arbitraryFloatPowerWrapper {
	for w := m.PowerWrappers; w != nil; w = w.Next {
		if w.id == id && w.Valid() {
			return w
		}
	}
	return nil
}
func (m *ArbitraryFloatManager) powerTwoWrapper(n uint32, negated bool) (*arbitraryFloatPowerWrapper, error) {
	return m.powerWrapper(n, false, negated)
}
func (m *ArbitraryFloatManager) powerTwoMinusOneWrapper(n uint32, negated bool) (*arbitraryFloatPowerWrapper, error) {
	return m.powerWrapper(n, true, negated)
}
func (m *ArbitraryFloatManager) powerWrapper(n uint32, minusOne, negated bool) (*arbitraryFloatPowerWrapper, error) {
	if m == nil || m.self != m || m.integer == nil || m.integer.self != m.integer {
		return nil, umemory.ErrRuntimeOwner
	}
	if !m.constructorComplete || m.closed || m.sourceEnded {
		return nil, arena.ErrClosed
	}
	if m.valueExecuting || m.integer.mpfBorrowedSourceCallbackActive() {
		return nil, ErrNumericPending
	}
	index := 0
	if minusOne {
		index = 2
	}
	if negated {
		index++
	}
	if id, found := m.powers[index].Get2(n); found {
		w := m.powerWrapperByID(id)
		if w == nil {
			return nil, umemory.ErrRuntimeOwner
		}
		return w, nil
	}
	if m.powerNextID == ^uint32(0) {
		return nil, arena.ErrBounds
	}
	header := uint64(reflect.TypeOf(arbitraryFloatPowerWrapper{}).Size())
	if e := m.budget.Reserve(header); e != nil {
		return nil, e
	}
	m.powerNextID++
	w := &arbitraryFloatPowerWrapper{manager: m, id: m.powerNextID, header: header, Next: m.PowerWrappers, invocation: 1}
	w.self = w
	m.PowerWrappers = w
	m.powerWrapperBytes += header
	w.tracePowerWrapperAllocated(minusOne)
	w.executing = true
	m.valueExecuting = true
	defer func() { w.executing = false; m.valueExecuting = false }()
	fail := func(e error) (*arbitraryFloatPowerWrapper, error) {
		if w.bodyCause == nil {
			w.bodyCause = e
		}
		return nil, e
	}
	block, e := m.memory.Allocate(16)
	w.Block = block
	w.original = block
	if e != nil {
		return fail(e)
	}
	for _, offset := range []uint64{0, 4, 8} {
		if e = m.memory.WriteUint32(block, offset, 0); e != nil {
			return fail(e)
		}
	}
	w.Initialized = true
	m.powers[index].Put(n, w.id)
	w.published = true
	out, e := w.Capture()
	if e != nil {
		return fail(e)
	}
	two := NewInt(2)
	context := integerAllocationInvocationContext{mpfPower: w, mpfPowerInvocation: w.invocation}
	if e = m.integer.powerWithPhysicalAllocationContext(&two, n, &out, nil, context, w.receiver()); e != nil {
		return fail(e)
	}
	if minusOne {
		minus := NewInt(-1)
		e = m.integer.addSubWithAllocationContext(&out, &minus, &out, false, context)
		// This actual positive-power decrement has no callback after output
		// publication: BIG output swaps a nonnull old cell into scratch; small
		// output performs no allocation. Scratch growth occurs before the swap.
		if out.IsSmall() {
			if write := w.receiver().publishCell(&out); write != nil {
				return fail(write)
			}
			if write := w.receiver().publishValue(&out); write != nil {
				return fail(write)
			}
		} else {
			if write := w.receiver().publishValue(&out); write != nil {
				return fail(write)
			}
			if write := w.receiver().publishCell(&out); write != nil {
				return fail(write)
			}
		}
		if e != nil {
			return fail(e)
		}
	}
	if negated {
		if e = m.integer.Neg(&out); e != nil {
			return fail(e)
		}
		if e = w.receiver().publishValue(&out); e != nil {
			return fail(e)
		}
	}
	return w, nil
}
func (c integerAllocationInvocationContext) mpfPowerRoots(m *IntegerManager) *NumericAllocationRoots {
	w := c.mpfPower
	if !w.Valid() || w.manager.powerWrapperByID(w.id) != w || w.manager.integer != m || !w.executing || !w.manager.valueExecuting || c.mpfPowerInvocation == 0 || c.mpfPowerInvocation != w.invocation || w.SourceDelDone || w.sourceFreeDone {
		return nil
	}
	return &w.Roots
}
func (w *arbitraryFloatPowerWrapper) destroy(own bool) error {
	if !w.Valid() {
		return umemory.ErrRuntimeOwner
	}
	if !own && w.sourceCause != nil {
		return w.sourceCause
	}
	stop := func(e error) error {
		if own {
			if w.ownCause == nil {
				w.ownCause = e
			}
		} else if w.sourceCause == nil {
			w.sourceCause = e
		}
		return e
	}
	if !w.SourceDelDone && w.Initialized && w.Block != (umemory.Block{}) {
		a, e := w.Capture()
		if e != nil {
			return stop(e)
		}
		if own {
			e = w.manager.integer.CleanupRawNumeral(&a)
		} else {
			e = w.manager.integer.Del(&a)
		}
		if write := w.receiver().publishCell(&a); write != nil {
			return stop(write)
		}
		if e != nil {
			return stop(e)
		}
		w.SourceDelDone = true
	}
	if own {
		if w.Roots.Cell.Object != (umemory.ObjectRef{}) {
			e := w.manager.integer.cleanupDetachedCopyCell(&w.Roots.Cell)
			if w.Roots.Cell.Object == (umemory.ObjectRef{}) {
				w.cellRetired = true
			}
			if e != nil {
				return stop(e)
			}
		}
		if w.Transferred.Block() != (umemory.Block{}) {
			if e := w.manager.integer.Allocator.CleanupTransferredAllocation(&w.Transferred); e != nil {
				return stop(e)
			}
		}
	}
	// Source memory_manager.h:112 skips nullptr. A failed wrapper new has
	// no Runtime payload to release; its cold metadata remains until owner end.
	if !w.sourceFreeDone && w.Block == (umemory.Block{}) {
		w.sourceFreeDone = true
	}
	if !w.sourceFreeDone {
		e := w.manager.memory.Deallocate(&w.Block)
		if w.Block == (umemory.Block{}) {
			w.sourceFreeDone = true
		}
		if e != nil {
			return stop(e)
		}
	}
	w.tracePowerWrapperDeallocated()
	return nil
}
func (m *ArbitraryFloatManager) destroyPowerWrappers(own bool) error {
	if own {
		for w := m.PowerWrappers; w != nil; w = w.Next {
			if e := w.destroy(true); e != nil {
				return e
			}
		}
		return nil
	}
	for i := range m.powers {
		for _, id := range m.powers[i].All() {
			w := m.powerWrapperByID(id)
			if w == nil {
				return umemory.ErrRuntimeOwner
			}
			if e := w.destroy(false); e != nil {
				return e
			}
		}
	}
	return nil
}
func (m *ArbitraryFloatManager) releasePowerWrapperHistory() {
	for w := m.PowerWrappers; w != nil; w = w.Next {
		m.budget.Release(w.header)
		w.header = 0
	}
	m.powerWrapperBytes = 0
}

// Called only at the actual allocating leaf return. It transfers the exact
// returned outcome; no before/after allocator census participates in authority.
func (m *IntegerManager) capturePowerAllocationOutcome(c integerAllocationInvocationContext, backing SourceCellBacking, outcome *umemory.SmallAllocatorAllocationOutcome) error {
	w := c.mpfPower
	if c.mpfPowerRoots(m) == nil {
		return ErrNumericCallerOwner
	}
	if outcome != nil && outcome.Pending != (umemory.Block{}) {
		if e := m.Allocator.TransferAllocationPending(outcome, &w.Transferred); e != nil {
			return e
		}
	}
	if backing.Object != (umemory.ObjectRef{}) {
		w.originalCell = backing
		m.PendingCell = SourceCellBacking{}
	}
	return nil
}
