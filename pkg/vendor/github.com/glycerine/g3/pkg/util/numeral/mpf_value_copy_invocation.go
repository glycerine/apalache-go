package numeral

// Source: src/util/mpf.cpp:316-333; src/util/scoped_numeral_vector.h:25-65.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// An opaque borrowed handle to one actual SET call. It carries no numeral or
// Runtime payload. Its child is issued only by the actual Integer allocation.
type ArbitraryFloatValueCopyReceipt struct {
	manager    *ArbitraryFloatManager
	frame      *arbitraryFloatValueCopyCall
	invocation uint64
	Child      integerCopyAllocationReceipt
}

type arbitraryFloatValueCopyCall struct {
	caller                        *ArbitraryFloatValueCopyChain
	callerNext                    *arbitraryFloatValueCopyCall
	callerFalseAck, callerTrueAck bool
	self                          *arbitraryFloatValueCopyCall
	manager                       *ArbitraryFloatManager
	Next                          *arbitraryFloatValueCopyCall
	Issued                        ArbitraryFloatValueCopyReceipt
	header                        uint64
	ChildFrame                    integerCopyAllocationScope
	executing, Retired            bool
	bodyCause, ownCause           error
}

// Embedded caller binding adds no acquisition to inline or reused-cell SET.
// Only the existing Integer allocation branch materializes a charged record.
type arbitraryFloatCopyBinding struct {
	chain             *ArbitraryFloatValueCopyChain
	conversionContext integerAllocationInvocationContext
	manager           *ArbitraryFloatManager
	sourceInteger     *IntegerManager
	invocation        uint64
	active            bool
	receipt           *ArbitraryFloatValueCopyReceipt
	record            *arbitraryFloatValueCopyCall
}

func (m *ArbitraryFloatManager) beginValueCopyBinding(receipt *ArbitraryFloatValueCopyReceipt, sourceInteger *IntegerManager) error {
	if sourceInteger == nil || sourceInteger.self != sourceInteger {
		return umemory.ErrRuntimeOwner
	}
	if sourceInteger.mpfValueRead != nil {
		return ErrNumericPending
	}
	if m.valueInvocation == ^uint64(0) {
		return ErrNumericPending
	}
	m.valueInvocation++
	m.valueBinding = arbitraryFloatCopyBinding{manager: m, sourceInteger: sourceInteger, invocation: m.valueInvocation, active: true, receipt: receipt}
	sourceInteger.mpfValueRead = &m.valueBinding
	if receipt != nil {
		*receipt = ArbitraryFloatValueCopyReceipt{}
	}
	return nil
}
func (z *IntegerManager) validateFloatCopyBinding(c *arbitraryFloatCopyBinding) error {
	if c == nil {
		return nil
	}
	m := c.manager
	if m == nil || m.self != m || m.integer != z || c != &m.valueBinding || !c.active || !m.valueExecuting || c.invocation == 0 || c.invocation != m.valueInvocation || m.closed || m.sourceEnded {
		return umemory.ErrRuntimeOwner
	}
	return nil
}
func (z *IntegerManager) prepareFloatCopyRecord(c *arbitraryFloatCopyBinding) error {
	if c == nil {
		return nil
	}
	if e := z.validateFloatCopyBinding(c); e != nil {
		return e
	}
	if e := c.conversionContext.prepareFormattedSetHistory(z); e != nil {
		return e
	}
	if e := c.conversionContext.prepareConversionHistory(z); e != nil {
		return e
	}
	if c.record != nil {
		return ErrNumericPending
	}
	m := c.manager
	bytes := uint64(reflect.TypeOf(arbitraryFloatValueCopyCall{}).Size())
	if e := m.budget.Reserve(bytes); e != nil {
		return e
	}
	q := &arbitraryFloatValueCopyCall{manager: m, Next: m.ValueCalls, header: bytes, executing: true}
	q.self = q
	if chain := c.chain; chain != nil {
		if chain.self != chain || chain.manager != m || !chain.executing || chain.ended {
			m.budget.Release(bytes)
			return umemory.ErrRuntimeOwner
		}
		q.caller = chain
		q.callerNext = chain.head
		chain.head = q
	}
	q.Issued = ArbitraryFloatValueCopyReceipt{manager: m, frame: q, invocation: c.invocation}
	c.record = q
	m.ValueCalls = q
	m.valueHistoryBytes += bytes
	return nil
}
func (z *IntegerManager) bindFloatCopyChild(c *arbitraryFloatCopyBinding, child integerCopyAllocationReceipt) {
	if c == nil {
		return
	}
	q := c.record
	q.Issued.Child = child
	if c.receipt != nil {
		*c.receipt = q.Issued
	}
}
func (m *ArbitraryFloatManager) finishValueCopyBinding(cause error) {
	c := &m.valueBinding
	if q := c.record; q != nil {
		q.bodyCause = cause
		q.executing = false
		q.Retired = q.Issued.Child.invocation == 0 || !m.integer.MatchesCopyAllocationInvocation(&q.Issued.Child)
		if c.receipt != nil {
			*c.receipt = q.Issued
		}
	}
	if c.sourceInteger != nil && c.sourceInteger.mpfValueRead == c {
		c.sourceInteger.mpfValueRead = nil
	}
	c.active = false
	c.receipt = nil
	c.record = nil
}

func (m *ArbitraryFloatManager) valueCopyCall(r *ArbitraryFloatValueCopyReceipt) (*arbitraryFloatValueCopyCall, error) {
	if m == nil || m.self != m || r == nil || r.manager != m || r.frame == nil || m.integer == nil || m.integer.self != m.integer {
		return nil, umemory.ErrRuntimeOwner
	}
	if m.closed {
		return nil, arena.ErrClosed
	}
	q := r.frame
	if q.self != q || q.manager != m || q.header == 0 || r.invocation == 0 || *r != q.Issued {
		return nil, umemory.ErrRuntimeOwner
	}
	for old := m.ValueCalls; old != nil; old = old.Next {
		if old == q {
			return q, nil
		}
	}
	return nil, umemory.ErrRuntimeOwner
}

// false acknowledges only completed children. true disposes the exact unjoined
// child after the caller has cleaned its older owning NUMs. Retired history
// never re-enters an allocator, even after that allocator frame is reused.
func (m *ArbitraryFloatManager) CleanupValueCopy(r *ArbitraryFloatValueCopyReceipt, abortUnjoined bool) error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	if m.valueExecuting || (m.integer != nil && m.integer.mpfBorrowedSourceCallbackActive()) {
		return ErrNumericPending
	}
	if r != nil && *r == (ArbitraryFloatValueCopyReceipt{}) {
		return nil
	}
	q, e := m.valueCopyCall(r)
	if e != nil {
		return e
	}
	if m.valueExecuting || q.executing {
		return ErrNumericPending
	}
	if q.Retired {
		return nil
	}
	child := &q.Issued.Child
	if child.manager != m.integer || !m.integer.validCopyAllocationScope(child.Frame) || child.invocation == 0 || child.Frame.invocation != child.invocation {
		return umemory.ErrRuntimeOwner
	}
	if child.Frame.Executing {
		return ErrNumericPending
	}
	if !child.Frame.active {
		q.Retired = true
		return nil
	}
	if !abortUnjoined {
		return nil
	}
	m.valueExecuting = true
	defer func() { m.valueExecuting = false }()
	e = m.integer.CleanupCopyAllocationInvocation(child)
	if e != nil {
		if q.ownCause == nil {
			q.ownCause = e
		}
		return e
	}
	q.Retired = true
	if m.valueCopy == *child {
		m.valueCopyAcknowledged = true
	}
	return nil
}

func (m *ArbitraryFloatManager) cleanupValueCopyCalls() error {
	for q := m.ValueCalls; q != nil; q = q.Next {
		if q.Retired {
			continue
		}
		if e := m.CleanupValueCopy(&q.Issued, true); e != nil {
			return e
		}
	}
	return nil
}
func (m *ArbitraryFloatManager) releaseValueCopyHistory() {
	for m.ValueCalls != nil {
		q := m.ValueCalls
		m.ValueCalls = q.Next
		m.budget.Release(q.header)
		m.valueHistoryBytes -= q.header
		q.header = 0
	}
}

// Charged caller metadata only. These bytes contain no owning NUM payload.
func (m *ArbitraryFloatManager) ValueCopyHistoryBytes() uint64 {
	if m == nil || m.self != m {
		panic(umemory.ErrRuntimeOwner)
	}
	return m.valueHistoryBytes
}

// This borrowed read link is installed by an explicit MPF SET caller. It
// protects that exact source during the destination's returning transport;
// it owns no source allocation and never scans a graph to infer ownership.
func (z *IntegerManager) mpfBorrowedSourceCallbackActive() bool {
	if z == nil || z.self != z {
		return false
	}
	c := z.mpfValueRead
	if c == nil || c.sourceInteger != z || !c.active || c.invocation == 0 {
		return false
	}
	m := c.manager
	return m != nil && m.self == m && c == &m.valueBinding && c.invocation == m.valueInvocation && m.valueExecuting && m.integer != nil && m.integer.self == m.integer && m.memory != nil && m.memory.FailureTransportActive()
}
