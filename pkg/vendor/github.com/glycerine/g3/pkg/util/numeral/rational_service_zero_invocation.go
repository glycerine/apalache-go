package numeral

// Source: src/util/rational.h:131-134,302-304.

import "sync/atomic"

// CopyZeroWithInvocation copies the borrowed static zero directly into the
// caller's physical value, without an intermediate owned rational.
func (s *RationalService) CopyZeroWithInvocation(dst *OwnedRational, c *RationalServiceInvocation) (e error) {
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
	return s.Manager.copyWithAllocationContext(&dst.Raw, &s.ZeroField, c.allocationContext())
}
