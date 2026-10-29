package numeral

// Source: src/util/mpf.cpp:69-106,114-181; src/util/mpz.h:156-164.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// The borrowed receipt names an actual allocation-call record. It contains no
// copied owning numeral. Inline input statements never materialize a record.
type ArbitraryFloatValueInputReceipt struct {
	Manager    *ArbitraryFloatManager
	Frame      *arbitraryFloatValueInputCall
	Invocation uint64
}
type arbitraryFloatValueInputCall struct {
	Self                *arbitraryFloatValueInputCall
	Manager             *ArbitraryFloatManager
	Next                *arbitraryFloatValueInputCall
	Issued              ArbitraryFloatValueInputReceipt
	header              uint64
	Numeric             NumericAllocationRoots
	Transferred         umemory.SmallAllocatorPendingAllocation
	originalCell        SourceCellBacking
	originalPending     umemory.Block
	cellRetired         bool
	executing, retired  bool
	bodyCause, ownCause error
}
type arbitraryFloatInputBinding struct {
	manager    *ArbitraryFloatManager
	invocation uint64
	active     bool
	record     *arbitraryFloatValueInputCall
	receipt    *ArbitraryFloatValueInputReceipt
}

func (m *ArbitraryFloatManager) beginValueInput(receipt *ArbitraryFloatValueInputReceipt) (integerAllocationInvocationContext, error) {
	if m.inputBinding.active || m.inputInvocation == ^uint64(0) {
		return integerAllocationInvocationContext{}, ErrNumericPending
	}
	m.inputInvocation++
	m.inputBinding = arbitraryFloatInputBinding{manager: m, invocation: m.inputInvocation, active: true, receipt: receipt}
	return integerAllocationInvocationContext{mpfInput: &m.inputBinding, mpfInputInvocation: m.inputInvocation}, nil
}
func (c integerAllocationInvocationContext) inputBinding(z *IntegerManager) *arbitraryFloatInputBinding {
	a := c.mpfInput
	if z == nil || z.self != z || a == nil || a.manager == nil || a.manager.self != a.manager {
		return nil
	}
	m := a.manager
	if m.integer != z || a != &m.inputBinding || c.mpfInputInvocation == 0 || c.mpfInputInvocation != a.invocation || a.invocation != m.inputInvocation || !a.active || !m.valueExecuting || m.closed || m.sourceEnded || z.Memory.FailureTransportActive() {
		return nil
	}
	return a
}
func (c integerAllocationInvocationContext) prepareInputAllocation(z *IntegerManager) error {
	if c.mpfInput == nil {
		return nil
	}
	a := c.inputBinding(z)
	if a == nil {
		return umemory.ErrRuntimeOwner
	}
	if a.record != nil {
		return nil
	}
	m := a.manager
	bytes := uint64(reflect.TypeOf(arbitraryFloatValueInputCall{}).Size())
	if e := m.budget.Reserve(bytes); e != nil {
		return e
	}
	q := &arbitraryFloatValueInputCall{Manager: m, Next: m.InputCalls, header: bytes, executing: true}
	q.Self = q
	q.Issued = ArbitraryFloatValueInputReceipt{m, q, a.invocation}
	m.InputCalls = q
	m.inputHistoryBytes += bytes
	a.record = q
	if a.receipt != nil {
		*a.receipt = q.Issued
	}
	return nil
}
func (c integerAllocationInvocationContext) inputRoots(z *IntegerManager) *NumericAllocationRoots {
	a := c.inputBinding(z)
	if a == nil || a.record == nil {
		return nil
	}
	q := a.record
	if q.Self != q || q.Manager != a.manager || q.Issued.Frame != q || q.Issued.Invocation != a.invocation || !q.executing {
		return nil
	}
	return &q.Numeric
}
func (z *IntegerManager) captureInputAllocationOutcome(c integerAllocationInvocationContext, backing SourceCellBacking, out *umemory.SmallAllocatorAllocationOutcome) error {
	a := c.inputBinding(z)
	if a == nil || a.record == nil {
		return nil
	}
	q := a.record
	if backing.Object != (umemory.ObjectRef{}) {
		q.originalCell = backing
		if z.PendingCell == backing {
			z.PendingCell = SourceCellBacking{}
		}
	}
	if out.Pending != (umemory.Block{}) {
		q.originalPending = out.Pending
		if e := z.Allocator.TransferAllocationPending(out, &q.Transferred); e != nil {
			return e
		}
		if q.Numeric.Allocator[0] == out.Pending {
			q.Numeric.Allocator[0] = umemory.Block{}
		}
	}
	return nil
}
func (m *ArbitraryFloatManager) finishValueInput(cause error) {
	a := &m.inputBinding
	if q := a.record; q != nil {
		q.executing = false
		q.bodyCause = cause
		q.retired = !NumericAllocationPending(&q.Numeric) && q.Transferred.Block() == (umemory.Block{})
	}
	a.active = false
}
func (m *ArbitraryFloatManager) valueInputCall(r *ArbitraryFloatValueInputReceipt) (*arbitraryFloatValueInputCall, error) {
	if m == nil || m.self != m || r == nil || r.Manager != m || r.Frame == nil || r.Invocation == 0 {
		return nil, umemory.ErrRuntimeOwner
	}
	for q := m.InputCalls; q != nil; q = q.Next {
		if q == r.Frame {
			if q.Self != q || q.Manager != m || q.Issued != *r {
				return nil, umemory.ErrRuntimeOwner
			}
			return q, nil
		}
	}
	return nil, umemory.ErrRuntimeOwner
}

// false reads only completion metadata; true releases the exact unjoined leaf
// after the caller owns its older passed MPF NUM. No output numeral is owned here.
func (m *ArbitraryFloatManager) CleanupValueInput(r *ArbitraryFloatValueInputReceipt, abort bool) error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	if m.valueExecuting || m.inputBinding.active || m.memory.FailureTransportActive() {
		return ErrNumericPending
	}
	if r == nil || *r == (ArbitraryFloatValueInputReceipt{}) {
		return nil
	}
	q, e := m.valueInputCall(r)
	if e != nil {
		return e
	}
	if q.executing {
		return ErrNumericPending
	}
	if q.retired || !abort {
		return nil
	}
	q.executing = true
	defer func() { q.executing = false }()
	e = m.integer.cleanupFormatAllocation(&q.Numeric, &q.Transferred, &q.cellRetired)
	if e != nil {
		if q.ownCause == nil {
			q.ownCause = e
		}
		return e
	}
	q.retired = true
	return nil
}
func (m *ArbitraryFloatManager) cleanupValueInputs() error {
	for q := m.InputCalls; q != nil; q = q.Next {
		if e := m.CleanupValueInput(&q.Issued, true); e != nil {
			return e
		}
	}
	return nil
}
func (m *ArbitraryFloatManager) releaseValueInputHistory() {
	for q := m.InputCalls; q != nil; q = q.Next {
		m.budget.Release(q.header)
		q.header = 0
	}
	m.inputHistoryBytes = 0
	m.InputCalls = nil
}
