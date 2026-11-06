package numeral

// Source: src/util/rational.h:43,306-308,462-464.

import "sync/atomic"

// CopyOneWithInvocation copies the borrowed static one directly into the
// caller's physical value, without an intermediate owned rational.
func (s *RationalService) CopyOneWithInvocation(dst *OwnedRational, c *RationalServiceInvocation) (e error) {
	if e = s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if e = s.rationalScalarOwner(dst); e != nil {
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
	return s.Manager.copyWithAllocationContext(&dst.Raw, &s.OneField, c.allocationContext())
}

// SubOneWithInvocation implements a - rational::one(): copy a, subtract the
// borrowed static constant in place, then copy the returned lvalue.
func (s *RationalService) SubOneWithInvocation(a *OwnedRational, c *RationalServiceInvocation) (v OwnedRational, e error) {
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
	defer s.endLexicalRational(&v, &e)
	if e = s.facadeEntry(a, rationalFacadeSub); e != nil {
		return
	}
	return s.facadeValueRawOperandsLockedWithInvocation(&a.Raw, &s.OneField, rationalFacadeSub, c)
}
