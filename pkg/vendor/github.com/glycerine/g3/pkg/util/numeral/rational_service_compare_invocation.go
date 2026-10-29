package numeral

// Source: src/util/rational.h:238-240,421-423; src/util/mpq.h:572-579;
// src/util/mpq.cpp:39-75.

import (
	"errors"
	"sync/atomic"
)

func (s *RationalService) LessWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (bool, error) {
	return s.lessWithInvocation(a, b, c)
}

func (s *RationalService) GreaterWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (bool, error) {
	// Source operator> delegates to operator< with operands reversed.
	return s.lessWithInvocation(b, a, c)
}

func (s *RationalService) lessWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (value bool, result error) {
	if result = s.lock(); result != nil {
		return
	}
	defer s.mu.Unlock()
	if result = s.rationalScalarOwner(a); result != nil {
		return
	}
	if result = s.rationalScalarOwner(b); result != nil {
		return
	}
	if result = s.prepareLexicalInvocation(c); result != nil {
		return
	}
	defer func() { result = s.finishLexicalInvocation(c, result) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if result = s.beginLexical(); result != nil {
		return
	}
	defer s.endLexicalError(&result)
	var comparison int
	comparison, result = s.Manager.compareWithAllocationContext(&a.Raw, &b.Raw, c.allocationContext())
	if cause := c.childSource(); cause != nil {
		if s.Lexical.Cause == nil {
			s.Lexical.Cause = cause
		}
		result = errors.Join(result, cause)
	}
	return comparison < 0, result
}
