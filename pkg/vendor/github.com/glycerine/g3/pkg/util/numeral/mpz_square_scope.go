package numeral

// Source: src/util/mpz.cpp:1871-1929; src/util/mpz.h:820.

import (
	"errors"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

var ErrPerfectSquarePending = errors.New("mpnum: retained perfect-square raw invocation")

// Actual six source RAW numerals, embedded in the charged Integer header. They
// have no automatic destructor: source Del is reached only at the end label.
type integerPerfectSquareInvocationReceipt struct {
	manager    *IntegerManager
	Frame      *integerPerfectSquareScope
	Invocation uint64
}

type integerPerfectSquareScope struct {
	owner                                    *IntegerManager
	Numeric                                  NumericAllocationRoots
	directReceipt                            integerPerfectSquareInvocationReceipt
	parentContext                            integerAllocationInvocationContext
	Executing, sourceExecuting, ownExecuting bool
	Values                                   [6]Int
	pending                                  umemory.ObjectRef
	Raw                                      [3]umemory.Block
	outputStarted                            bool
	outputChoice                             uint8
	Generation                               uint64
	Active, EndEntered, stopped              bool
	sourceIndex, cleanupIndex                uint8
	BodyCause, sourceCause, cleanupCause     error
}

func (m *IntegerManager) finishPerfectSquare() error {
	s := &m.PerfectSquare
	if s.directReceipt.Invocation != 0 {
		if !s.Executing || s.sourceExecuting || s.ownExecuting {
			return ErrNumericPending
		}
		s.sourceExecuting = true
		defer func() { s.sourceExecuting = false }()
	}
	if s.stopped {
		return s.sourceCause
	}
	if !s.Active || !s.EndEntered {
		return nil
	}
	for s.sourceIndex < 6 {
		if _, e := SourceNumeralDel(m, &s.Values[s.sourceIndex]); e != nil {
			s.stopped = true
			s.sourceCause = e
			return e
		}
		s.sourceIndex++
	}
	s.Active = false
	return nil
}

// CleanupPerfectSquare is an explicit owning abort, never a source end-label
// execution. It resumes only these actual six raw fields in ascending order.
func (m *IntegerManager) CleanupPerfectSquare() error {
	if m == nil || m.self != m {
		return umemory.ErrRuntimeOwner
	}
	s := &m.PerfectSquare
	if s.directReceipt.Invocation != 0 {
		return m.CleanupPerfectSquareInvocation(&s.directReceipt, true)
	}
	if s.directReceipt.Invocation != 0 && (s.Executing || s.sourceExecuting || s.ownExecuting) {
		return ErrNumericPending
	}
	if !s.Active {
		return nil
	}
	for s.cleanupIndex < 6 {
		p := &s.Values[s.cleanupIndex]
		var e error
		if m.SourceFinalized || m.ConstructorUnwound {
			if p.Cell != 0 && !m.IsFailureRef(p.Cell) {
				e = m.freeAfterSourceFinalize(p.Cell)
			}
			if e == nil {
				p.Cell = 0
			}
		} else {
			e = m.Del(p)
		}
		if e != nil {
			s.cleanupCause = e
			return e
		}
		s.cleanupIndex++
	}
	if s.pending != (umemory.ObjectRef{}) {
		if m.PendingCell.Object == s.pending {
			if e := m.cleanupPendingAllocation(); e != nil {
				s.cleanupCause = e
				return e
			}
		}
		s.pending = umemory.ObjectRef{}
	}
	if s.Raw != ([3]umemory.Block{}) {
		now, e := m.Allocator.PendingRoots()
		if e != nil {
			return e
		}
		if now == s.Raw {
			e = m.Allocator.CleanupPending()
			after, x := m.Allocator.PendingRoots()
			s.Raw = after
			if e != nil {
				s.cleanupCause = e
				return errors.Join(e, x)
			}
			if x != nil {
				return x
			}
		}
		s.Raw = [3]umemory.Block{}
	}
	s.Active = false
	s.cleanupCause = nil
	return nil
}

// Only actual internal RAW-local operations acquire pending physical roots.
// Final SET(root,lo/hi) is caller-owned and deliberately bypasses this adapter.
func (m *IntegerManager) perfectSquareCall(op uint8, a, b, out *Int) (cause error) {
	beforeCell := m.PendingCell
	before := beforeCell.Object
	raw, e := m.Allocator.PendingRoots()
	if e != nil {
		return e
	}
	switch op {
	case 0:
		cause = m.Copy(out, a)
	case 1:
		cause = m.Add(a, b, out)
	case 2:
		cause = m.Mul(a, b, out)
	case 3:
		cause = m.Div(a, b, out)
	}
	s := &m.PerfectSquare
	if before == (umemory.ObjectRef{}) && m.PendingCell.Object != (umemory.ObjectRef{}) {
		s.pending = m.PendingCell.Object
	}
	after, e := m.Allocator.PendingRoots()
	if raw == ([3]umemory.Block{}) && after != ([3]umemory.Block{}) {
		s.Raw = after
	}
	if e != nil {
		return errors.Join(cause, e)
	}
	// A directly issued receipt uses the same exact allocation authority as
	// the ordinary six-RAW route; its cleanup must retain these actual tokens.
	if e := s.Numeric.After(m, NumericAllocationRoots{Cell: beforeCell, Allocator: raw}); e != nil {
		return errors.Join(cause, e)
	}
	return cause
}

func (m *IntegerManager) beginPerfectSquareRawScope() error {
	return m.beginPerfectSquareRawScopeWithContext(integerAllocationInvocationContext{})
}
func (m *IntegerManager) beginPerfectSquareRawScopeWithContext(context integerAllocationInvocationContext) error {
	s := &m.PerfectSquare
	if s.Active {
		return ErrPerfectSquarePending
	}
	if m.PerfectSquareGeneration == ^uint64(0) || (context != (integerAllocationInvocationContext{}) && context.roots(m) == nil) {
		return ErrNumericPending
	}
	m.PerfectSquareGeneration++
	*s = integerPerfectSquareScope{owner: m, Active: true, Generation: m.PerfectSquareGeneration, parentContext: context}
	if context != (integerAllocationInvocationContext{}) {
		s.directReceipt = integerPerfectSquareInvocationReceipt{m, s, s.Generation}
		context.recordCaller(m, "perfect-square", NumericInvocationReceipt{PerfectSquare: &s.directReceipt})
	}
	return nil
}
func (m *IntegerManager) endPerfectSquareRawScope() error {
	m.PerfectSquare.EndEntered = true
	return m.finishPerfectSquare()
}

// Actual direct allocation identity is stamped at the real six-RAW start.
func (m *IntegerManager) MatchesPerfectSquareHistory(r *integerPerfectSquareInvocationReceipt) bool {
	return m != nil && m.self == m && r != nil && r.manager == m && r.Frame == &m.PerfectSquare && r.Frame.owner == m && r.Invocation != 0 && r.Invocation == r.Frame.Generation && r.Frame.directReceipt == *r
}
func (m *IntegerManager) MatchesPerfectSquareInvocation(r *integerPerfectSquareInvocationReceipt) bool {
	return m.MatchesPerfectSquareHistory(r) && r.Frame.Active
}
func (m *IntegerManager) perfectSquareContextRoots(c integerAllocationInvocationContext) *NumericAllocationRoots {
	r := c.perfectSquare
	if !m.MatchesPerfectSquareInvocation(r) || c.perfectSquareInvocation != r.Invocation || !r.Frame.Executing || r.Frame.sourceExecuting || r.Frame.ownExecuting {
		return nil
	}
	if r.Frame.parentContext != (integerAllocationInvocationContext{}) && r.Frame.parentContext.roots(m) == nil {
		return nil
	}
	return &r.Frame.Numeric
}
func (m *IntegerManager) perfectSquareCallWithContext(op uint8, a, b, out *Int, context integerAllocationInvocationContext) (cause error) {
	if context == (integerAllocationInvocationContext{}) {
		return m.perfectSquareCall(op, a, b, out)
	}
	if m.perfectSquareContextRoots(context) == nil {
		return ErrNumericPending
	}
	switch op {
	case 0:
		cause = m.copyFromOwnerWithAllocationContext(out, m, a, nil, context)
	case 1:
		cause = m.addSubWithAllocationContext(a, b, out, false, context)
	case 2:
		cause = m.mulWithAllocationContext(a, b, out, context)
	case 3:
		cause = m.divWithAllocationContext(a, b, out, context)
	}
	s := &m.PerfectSquare
	if s.sourceCause == nil && s.parentContext.Caller != nil {
		s.sourceCause = s.parentContext.Caller.callSourceCause(m)
	}
	return cause
}
func (m *IntegerManager) perfectSquareInvocationSourceCause(r *integerPerfectSquareInvocationReceipt) error {
	if !m.MatchesPerfectSquareHistory(r) {
		return nil
	}
	return r.Frame.sourceCause
}
func (m *IntegerManager) CleanupPerfectSquareInvocation(r *integerPerfectSquareInvocationReceipt, abort bool) (cause error) {
	if !m.MatchesPerfectSquareInvocation(r) {
		return nil
	}
	s := r.Frame
	if s.Executing || s.sourceExecuting || s.ownExecuting {
		return ErrNumericPending
	}
	if !abort && !s.EndEntered {
		return nil
	}
	s.ownExecuting = true
	defer func() {
		s.ownExecuting = false
		if cause != nil && s.cleanupCause == nil {
			s.cleanupCause = cause
		}
	}()
	for s.cleanupIndex < 6 {
		if e := m.CleanupRawNumeral(&s.Values[s.cleanupIndex]); e != nil {
			return e
		}
		s.cleanupIndex++
	}
	if abort {
		if e := CleanupNumericAllocationRoots(m, &s.Numeric); e != nil {
			return e
		}
		s.Active = false
	}
	return nil
}
