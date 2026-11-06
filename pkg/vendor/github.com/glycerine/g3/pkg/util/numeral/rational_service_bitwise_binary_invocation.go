package numeral

// Source: src/util/rational.h:370-386; src/util/mpq.h:632-655;
// src/util/mpz.cpp:1018-1148.

import "sync/atomic"

func (s *RationalService) BitwiseOrWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (OwnedRational, error) {
	return s.bitwiseBinaryWithInvocation(a, b, c, 1)
}
func (s *RationalService) BitwiseAndWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (OwnedRational, error) {
	return s.bitwiseBinaryWithInvocation(a, b, c, 2)
}
func (s *RationalService) BitwiseXorWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation) (OwnedRational, error) {
	return s.bitwiseBinaryWithInvocation(a, b, c, 3)
}
func (s *RationalService) bitwiseBinaryWithInvocation(a, b *OwnedRational, c *RationalServiceInvocation, op uint8) (v OwnedRational, e error) {
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
	return s.extraBinaryLockedWithInvocation(a, b, op, c)
}
