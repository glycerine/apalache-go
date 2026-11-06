package numeral

// Source: src/util/rational.cpp:50-62; src/util/rational.h; src/util/scoped_numeral.h.

import (
	"errors"
	"math"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// Embedded in the actual charged caller header. The roots below are transferred
// at the reached service epilogue, never copied from an observer/census. No
// separate per-call header or inline numerical reservation is introduced.
type RationalServiceInvocation struct {
	self                                *RationalServiceInvocation
	service                             *RationalService
	manager                             *RationalManager
	integer                             *IntegerManager
	borrowedInteger                     *IntegerManager
	next                                *RationalServiceInvocation
	characters                          integerCharacterScope
	characterFormatting                 naturalFormattingScope
	insertion                           NumeralInsertionReceipt
	scope                               rationalLexicalScope
	numeric                             NumericAllocationRoots
	division, gcd, extended, bitwiseNot integerRawInvocationReceipt
	bitwiseOr, bitwiseAnd, bitwiseXor   integerRawInvocationReceipt
	addGeneration, normGeneration       uint64
	compareGeneration, powerGeneration  uint64
	transferred                         umemory.SmallAllocatorPendingAllocation
	originalCell                        SourceCellBacking
	originalPending                     umemory.Block
	cellRetired                         bool

	invocation                            uint64
	executing, ended, falseDone, trueDone bool
	bodyCause, sourceCause, ownCause      error
}

func (c *RationalServiceInvocation) valid() bool {
	if c == nil || c.self != c || c.service == nil || c.service.self != c.service || c.manager == nil || c.manager.self != c.manager || c.integer == nil || c.integer.self != c.integer || c.manager.Z != c.integer || c.invocation == 0 || c.ended {
		return false
	}
	for n := c.service.lexicalInvocations; n != nil; n = n.next {
		if n == c {
			return true
		}
	}
	return false
}
func (c *RationalServiceInvocation) SourceReady() error {
	if c != nil && c.self == nil && c.service == nil && c.invocation == 0 {
		return nil
	}
	if !c.valid() {
		return arena.ErrInvalidSlot
	}
	if c.executing || c.service.runtimeTransportExecuting() {
		return ErrRationalLexicalPending
	}
	return c.sourceCause
}
func (c *RationalServiceInvocation) Causes() (body, source, own error) {
	if c == nil || c.self != c {
		return arena.ErrInvalidSlot, nil, nil
	}
	return c.bodyCause, c.sourceCause, c.ownCause
}
func (c *RationalServiceInvocation) cleanupLocked(abort bool) error {
	if !c.valid() {
		return arena.ErrInvalidSlot
	}
	if c.executing || c.scope.depth != 0 {
		return ErrRationalLexicalPending
	}
	if abort && !c.falseDone {
		return ErrRationalLexicalPending
	}
	if !abort && c.falseDone || abort && c.trueDone {
		return nil
	}
	if c.insertion.active {
		if e := completeServiceNumeralInsertion(&c.insertion, c.service, abort); e != nil {
			if c.ownCause == nil {
				c.ownCause = e
			}
			return e
		}
	}
	if !abort {
		if e := c.cleanupRationalRaw(false); e != nil {
			if c.ownCause == nil {
				c.ownCause = e
			}
			return e
		}
	}
	if !abort {
		for _, r := range []*integerRawInvocationReceipt{&c.extended, &c.division, &c.gcd, &c.bitwiseNot, &c.bitwiseOr, &c.bitwiseAnd, &c.bitwiseXor} {
			if e := c.integer.CleanupRawInvocation(r, false); e != nil {
				if c.ownCause == nil {
					c.ownCause = e
				}
				return e
			}
		}
	}
	for i := uint8(0); i < c.scope.Count; i++ {
		if c.scope.Done&(1<<i) != 0 || c.scope.abort[i] != abort {
			continue
		}
		if e := cleanupRationalInvocationValue(c.integer, &c.scope.Roots[i], &c.scope.components[i]); e != nil {
			if c.ownCause == nil {
				c.ownCause = e
			}
			return e
		}
		c.scope.Done |= 1 << i
	}
	if abort {
		if e := c.cleanupRationalRaw(true); e != nil {
			if c.ownCause == nil {
				c.ownCause = e
			}
			return e
		}
		for _, r := range []*integerRawInvocationReceipt{&c.extended, &c.division, &c.gcd, &c.bitwiseNot, &c.bitwiseOr, &c.bitwiseAnd, &c.bitwiseXor} {
			if e := c.integer.CleanupRawInvocation(r, true); e != nil {
				if c.ownCause == nil {
					c.ownCause = e
				}
				return e
			}
		}
		if e := c.integer.cleanupFormatAllocation(&c.numeric, &c.transferred, &c.cellRetired); e != nil {
			if c.ownCause == nil {
				c.ownCause = e
			}
			return e
		}
		c.trueDone = true
	} else {
		c.falseDone = true
	}
	return nil
}
func (c *RationalServiceInvocation) Cleanup(abort bool) error {
	if c != nil && c.self == nil && c.service == nil && c.invocation == 0 {
		return nil
	}
	if !c.valid() {
		return arena.ErrInvalidSlot
	}
	if c.service.sourceInsertionExecuting || c.service.runtimeTransportExecuting() {
		return arena.ErrInvalidSlot
	}
	c.service.mu.Lock()
	defer c.service.mu.Unlock()
	return c.cleanupLocked(abort)
}
func (c *RationalServiceInvocation) EndContext() error {
	if c != nil && c.self == nil && c.service == nil && c.invocation == 0 {
		return nil
	}
	if !c.valid() {
		return arena.ErrInvalidSlot
	}
	if c.service.sourceInsertionExecuting || c.service.runtimeTransportExecuting() {
		return arena.ErrInvalidSlot
	}
	c.service.mu.Lock()
	defer c.service.mu.Unlock()
	if c.executing || !c.falseDone || !c.trueDone {
		return ErrRationalLexicalPending
	}
	link := &c.service.lexicalInvocations
	for *link != c {
		if *link == nil {
			return arena.ErrInvalidSlot
		}
		link = &(*link).next
	}
	*link = c.next
	c.ended = true
	return nil
}
func (s *RationalService) prepareLexicalInvocation(c *RationalServiceInvocation) error {
	if c == nil || c.self != nil || s.lexicalPrepared != nil || s.lexicalExecuting != nil {
		return arena.ErrInvalidSlot
	}
	if s.lexicalSerial == math.MaxUint64 {
		return arena.ErrBounds
	}
	if e := s.lexicalReady(); e != nil {
		return e
	}
	s.lexicalPrepared = c
	return nil
}

// Called only by actual outer beginLexical, after existing owner/ready guards.
func (s *RationalService) enterLexicalInvocation() {
	c := s.lexicalPrepared
	if c == nil || s.Lexical.depth != 0 {
		return
	}
	s.lexicalSerial++
	*c = RationalServiceInvocation{service: s, manager: s.Manager, integer: s.Manager.Z, invocation: s.lexicalSerial, executing: true, next: s.lexicalInvocations}
	c.self = c
	s.lexicalInvocations = c
	s.lexicalExecuting = c
	s.lexicalPrepared = nil
}
func (s *RationalService) finishLexicalInvocation(c *RationalServiceInvocation, body error) error {
	s.lexicalPrepared = nil
	if s.lexicalExecuting != c {
		return body
	}
	// This is the reached owning transfer of this exact invocation's stack, not
	// a reconstruction from roots/counts or a manager-wide cleanup permission.
	c.scope = s.Lexical
	s.Lexical = rationalLexicalScope{}
	c.bodyCause = body
	c.sourceCause = c.scope.Cause
	c.executing = false
	s.lexicalExecuting = nil
	if c.sourceCause != nil && !errors.Is(body, c.sourceCause) {
		return errors.Join(body, c.sourceCause)
	}
	return body
}
func (s *RationalService) ModWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (OwnedRational, error) {
	return s.valueWithInvocation(a, b, c, rationalFacadeMod)
}
func (s *RationalService) AddWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (OwnedRational, error) {
	return s.valueWithInvocation(a, b, c, rationalFacadeAdd)
}

// Source: src/util/rational.h:462-464. Copy, subtract in place, then return copy.
func (s *RationalService) SubWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (OwnedRational, error) {
	return s.valueWithInvocation(a, b, c, rationalFacadeSub)
}
func (s *RationalService) MulWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (OwnedRational, error) {
	return s.valueWithInvocation(a, b, c, rationalFacadeMul)
}
func (s *RationalService) DivWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (OwnedRational, error) {
	return s.valueWithInvocation(a, b, c, rationalFacadeDiv)
}

// Source: src/util/rational.h:519-523. The result is returned by value; x and y
// publish in the original manager order, including prefixes before failure.
func (s *RationalService) GCDCoefficientsWithInvocation(a, b, x, y *OwnedRational, c *RationalServiceInvocation) (v OwnedRational, e error) {
	if e = s.lock(); e != nil {
		return
	}
	defer s.mu.Unlock()
	if e = s.prepareLexicalInvocation(c); e != nil {
		return
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	return s.gcdCoefficientsLockedWithInvocation(a, b, x, y, c)
}
func (s *RationalService) valueWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation, op rationalFacadeBinaryOp) (v OwnedRational, e error) {
	if e = s.lock(); e != nil {
		return
	}
	defer s.mu.Unlock()
	if e = s.prepareLexicalInvocation(c); e != nil {
		return
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	return s.facadeValueLockedWithInvocation(a, b, op, c)
}
func (s *RationalService) PowerOfTwoWithInvocation(k uint32, c *RationalServiceInvocation) (v OwnedRational, e error) {
	if e = s.lock(); e != nil {
		return
	}
	defer s.mu.Unlock()
	if e = s.prepareLexicalInvocation(c); e != nil {
		return
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	return s.powerOfTwoLockedWithInvocation(k, c)
}
func (s *RationalService) lexicalInvocationSourceReady() error {
	for c := s.lexicalInvocations; c != nil; c = c.next {
		if !c.valid() {
			return arena.ErrInvalidSlot
		}
		if c.executing {
			return ErrRationalLexicalPending
		}
		if c.sourceCause != nil {
			return errors.Join(ErrRationalLexicalPending, c.sourceCause)
		}
	}
	return nil
}
func (s *RationalService) cleanupLexicalInvocations() error {
	for c := s.lexicalInvocations; c != nil; c = c.next {
		if e := c.cleanupLocked(false); e != nil {
			return e
		}
		if e := c.cleanupLocked(true); e != nil {
			return e
		}
	}
	return nil
}

func cleanupRationalInvocationValue(z *IntegerManager, q *Rational, component *uint8) error {
	for *component < 2 {
		p := &q.Num
		if *component == 1 {
			p = &q.Den
		}
		if e := z.CleanupRawNumeral(p); e != nil {
			return e
		}
		*component++
	}
	*q = Rational{}
	return nil
}

func (s *RationalService) CopyWithInvocation(dst, src *OwnedRational, c *RationalServiceInvocation) (e error) {
	if e = s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e = s.rationalScalarOwner(dst); e != nil {
		return e
	}
	if e = s.rationalScalarOwner(src); e != nil {
		return e
	}
	if e = s.prepareLexicalInvocation(c); e != nil {
		return e
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e = s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&e)
	return s.copyLockedWithContext(dst, src, c.allocationContext())
}

// Source: src/util/rational.h:242-244; src/util/mpq.h:201;
// src/util/mpz.h:499-510.
// NegAssignWithInvocation preserves the caller's physical numerator. Only the
// inline INT_MIN branch reaches a signed setter and its actual allocation.
func (s *RationalService) NegAssignWithInvocation(a *OwnedRational, c *RationalServiceInvocation) (e error) {
	if e = s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e = s.rationalScalarOwner(a); e != nil {
		return e
	}
	if e = s.prepareLexicalInvocation(c); e != nil {
		return e
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e = s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&e)
	return s.Manager.negWithSourceContext(&a.Raw, c.allocationContext())
}

// Same source compound operator; the caller's NUM is the physical receiver.
func (s *RationalService) AddAssignWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) error {
	return s.compoundWithInvocation(a, b, c, rationalFacadeAdd)
}
func (s *RationalService) SubAssignWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) error {
	return s.compoundWithInvocation(a, b, c, rationalFacadeSub)
}

// MulAssignWithInvocation uses the physical caller receiver for both NUM and
// DEN, retaining the original operator*= component publication order.
// Source: src/util/rational.h:161-164; src/util/mpq.h:324-332.
func (s *RationalService) MulAssignWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) error {
	return s.compoundWithInvocation(a, b, c, rationalFacadeMul)
}
func (s *RationalService) compoundWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation, op rationalFacadeBinaryOp) (e error) {
	if e = s.lock(); e != nil {
		return
	}
	defer s.mu.Unlock()
	if e = s.prepareLexicalInvocation(c); e != nil {
		return
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e = s.beginLexical(); e != nil {
		return
	}
	defer s.endLexicalError(&e)
	if e = s.rationalScalarOwner(a); e != nil {
		return
	}
	if e = s.rationalScalarOwner(b); e != nil {
		return
	}
	if op == rationalFacadeMul {
		e = s.Manager.mulWithAllocationContext(&a.Raw, &b.Raw, &a.Raw, c.allocationContext())
	} else {
		e = s.Manager.addSubWithAllocationContext(&a.Raw, &b.Raw, &a.Raw, op == rationalFacadeSub, c.allocationContext())
	}
	if cause := c.childSource(); cause != nil {
		if s.Lexical.Cause == nil {
			s.Lexical.Cause = cause
		}
		e = errors.Join(e, cause)
	}
	return
}

func (s *RationalService) RemWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (OwnedRational, error) {
	return s.valueWithInvocation(a, b, c, rationalFacadeRem)
}
func (s *RationalService) IDivWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (OwnedRational, error) {
	return s.valueWithInvocation(a, b, c, rationalFacadeIDiv)
}

// Source: src/util/rational.h:186-190. Delegate original truncating machine
// integer division with its actual returned-value child allocation context.
func (s *RationalService) MachineDivWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (OwnedRational, error) {
	return s.valueWithInvocation(a, b, c, rationalFacadeMachineDiv)
}

// Source rational(mpz const&) constructs NUM from the explicitly supplied
// Integer owner, then reset_denominator publishes DEN. The source NUM is borrowed.
func (s *RationalService) SetIntegerFromOwnerWithInvocation(out *OwnedRational, source *IntegerManager, n *Int, c *RationalServiceInvocation) (e error) {
	if e = s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e = s.rationalScalarOwner(out); e != nil {
		return e
	}
	if source == nil || source.self != source {
		return umemory.ErrRuntimeOwner
	}
	if e = source.check(n); e != nil {
		return e
	}
	if source.rationalServiceRead != nil {
		return ErrNumericPending
	}
	if e = s.prepareLexicalInvocation(c); e != nil {
		return e
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e = s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&e)
	c.borrowedInteger = source
	source.rationalServiceRead = c
	defer func() {
		if source.rationalServiceRead == c {
			source.rationalServiceRead = nil
		}
	}()
	return s.Manager.setIntegerFromOwnerWithAllocationContext(&out.Raw, source, n, c.allocationContext())
}

// Lifetime exclusion only: this exact borrowed source is protected solely
// during the destination's actual returning allocation transport.
func (z *IntegerManager) rationalServiceReadCallbackActive() bool {
	if z == nil || z.self != z {
		return false
	}
	c := z.rationalServiceRead
	return c != nil && c.valid() && c.borrowedInteger == z && c.executing && c.integer.Memory.FailureTransportActive()
}

// Source rational(int64) reaches NUM SET before resetting DEN.
func (s *RationalService) SetInt64WithInvocation(out *OwnedRational, n int64, c *RationalServiceInvocation) (e error) {
	if e = s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e = s.rationalScalarOwner(out); e != nil {
		return e
	}
	if e = s.prepareLexicalInvocation(c); e != nil {
		return e
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e = s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&e)
	return s.Manager.setInt64WithAllocationContext(&out.Raw, n, c.allocationContext())
}

// Source: src/util/rational.h:48; src/util/mpq.h:712-715.
func (s *RationalService) SetUnsignedWithInvocation(out *OwnedRational, n uint32, c *RationalServiceInvocation) (e error) {
	if e = s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e = s.rationalScalarOwner(out); e != nil {
		return e
	}
	if e = s.prepareLexicalInvocation(c); e != nil {
		return e
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e = s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&e)
	return s.Manager.setUnsignedWithAllocationContext(&out.Raw, n, c.allocationContext())
}

// Source: src/util/rational.h:65; src/util/mpq.h:set.
func (s *RationalService) SetUint64WithInvocation(out *OwnedRational, n uint64, c *RationalServiceInvocation) (e error) {
	if e = s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e = s.rationalScalarOwner(out); e != nil {
		return e
	}
	if e = s.prepareLexicalInvocation(c); e != nil {
		return e
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e = s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&e)
	return s.Manager.setUint64WithAllocationContext(&out.Raw, n, c.allocationContext())
}

// Source: src/util/rational.h:198-200; src/util/mpq.h:549-553.
// The void overload writes the caller's physical receiver directly. It does
// not construct a returned rational or copy a temporary into the receiver.
func (s *RationalService) ModIntoWithInvocation(a, b, out *OwnedRational, c *RationalServiceInvocation) (e error) {
	if e = s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	for _, value := range []*OwnedRational{a, b, out} {
		if e = s.rationalScalarOwner(value); e != nil {
			return e
		}
	}
	if e = s.prepareLexicalInvocation(c); e != nil {
		return e
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e = s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&e)
	e = s.Manager.modWithAllocationContext(&a.Raw, &b.Raw, &out.Raw, c.allocationContext())
	if cause := c.childSource(); cause != nil {
		if s.Lexical.Cause == nil {
			s.Lexical.Cause = cause
		}
		e = errors.Join(e, cause)
	}
	return e
}

// Source: src/util/rational.h:182-184; src/util/mpq.h:523-527.
// The void overload writes the caller's physical receiver directly. It does
// not construct a returned rational or copy a temporary into the receiver.
func (s *RationalService) IDivIntoWithInvocation(a, b, out *OwnedRational, c *RationalServiceInvocation) (e error) {
	if e = s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	for _, value := range []*OwnedRational{a, b, out} {
		if e = s.rationalScalarOwner(value); e != nil {
			return e
		}
	}
	if e = s.prepareLexicalInvocation(c); e != nil {
		return e
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e = s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&e)
	e = s.Manager.integerOpRationalWithAllocationContext(&a.Raw, &b.Raw, &out.Raw, qIntegerDiv, c.allocationContext())
	if cause := c.childSource(); cause != nil {
		if s.Lexical.Cause == nil {
			s.Lexical.Cause = cause
		}
		e = errors.Join(e, cause)
	}
	return e
}

// Source: src/util/rational.h:220-225. Copy old value before mutating receiver;
// retain the distinct returned old value even when the caller discards it.
func (s *RationalService) PostIncWithInvocation(a *OwnedRational, c *RationalServiceInvocation) (v OwnedRational, e error) {
	if e = s.lock(); e != nil {
		return
	}
	defer s.mu.Unlock()
	if e = s.prepareLexicalInvocation(c); e != nil {
		return
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	return s.facadePostfixLockedWithInvocation(a, false, c)
}

// Source: src/util/rational.h:474-478. Input stays unchanged.
func (s *RationalService) NegWithInvocation(a *OwnedRational, c *RationalServiceInvocation) (v OwnedRational, e error) {
	if e = s.lock(); e != nil {
		return
	}
	defer s.mu.Unlock()
	if e = s.prepareLexicalInvocation(c); e != nil {
		return
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	return s.facadeUnaryLockedWithInvocation(a, facadeNeg, 0, c)
}

// Source: src/util/rational.h:171-174. Preserve the original physical
// receiver for operator%= without constructing a returned rational.
func (s *RationalService) RemAssignWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (e error) {
	if e = s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	for _, value := range []*OwnedRational{a, b} {
		if e = s.rationalScalarOwner(value); e != nil {
			return e
		}
	}
	if e = s.prepareLexicalInvocation(c); e != nil {
		return e
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e = s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&e)
	e = s.Manager.integerOpRationalWithAllocationContext(&a.Raw, &b.Raw, &a.Raw, qIntegerRem, c.allocationContext())
	if cause := c.childSource(); cause != nil {
		if s.Lexical.Cause == nil {
			s.Lexical.Cause = cause
		}
		e = errors.Join(e, cause)
	}
	return e
}

// Source: src/util/rational.h:507-511. Copy input, apply original abs and return.
func (s *RationalService) AbsWithInvocation(a *OwnedRational, c *RationalServiceInvocation) (v OwnedRational, e error) {
	if e = s.lock(); e != nil {
		return
	}
	defer s.mu.Unlock()
	if e = s.prepareLexicalInvocation(c); e != nil {
		return
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	return s.facadeUnaryLockedWithInvocation(a, facadeAbs, 0, c)
}

// Source: src/util/rational.h:503-505; src/util/mpq.cpp:282-295.
// Delegate original repeated squaring and its actual RAW power child.
func (s *RationalService) PowerWithInvocation(a *OwnedRational, exponent uint32, c *RationalServiceInvocation) (v OwnedRational, e error) {
	if e = s.lock(); e != nil {
		return
	}
	defer s.mu.Unlock()
	if e = s.prepareLexicalInvocation(c); e != nil {
		return
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	return s.facadeUnaryLockedWithInvocation(a, facadePower, exponent, c)
}

// Source: src/util/rational.h:388-392.
func (s *RationalService) BitwiseNotWithInvocation(width uint32, a *OwnedRational, c *RationalServiceInvocation) (v OwnedRational, e error) {
	if e = s.lock(); e != nil {
		return
	}
	defer s.mu.Unlock()
	if e = s.prepareLexicalInvocation(c); e != nil {
		return
	}
	defer func() { e = s.finishLexicalInvocation(c, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	return s.bitwiseNotLockedWithInvocation(width, a, c)
}
