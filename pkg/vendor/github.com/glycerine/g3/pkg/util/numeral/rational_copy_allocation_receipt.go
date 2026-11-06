package numeral

// Source: src/util/mpq.h copy; src/util/mpz.cpp big_set allocation.

import (
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"
)

// RationalCopyAllocationReceipt identifies the exact integer allocation
// reached by a rational copy. It owns only a retained allocation prefix, not
// either rational operand.
type RationalCopyAllocationReceipt struct {
	service *RationalService
	child   integerCopyAllocationReceipt
}

func (r *RationalCopyAllocationReceipt) Active() bool {
	return r != nil && r.service != nil && r.service.Manager != nil && r.service.Manager.Z.MatchesCopyAllocationInvocation(&r.child)
}

func (s *RationalService) CopyWithAllocationReceipt(dst, src *OwnedRational, r *RationalCopyAllocationReceipt) error {
	if r == nil {
		return arena.ErrInvalidSlot
	}
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if r.Active() {
		return ErrNumericPending
	}
	*r = RationalCopyAllocationReceipt{service: s}
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e := s.rationalScalarOwner(dst); e != nil {
		return e
	}
	if e := s.rationalScalarOwner(src); e != nil {
		return e
	}
	return s.Manager.copyWithAllocationReceipt(&dst.Raw, &src.Raw, integerAllocationInvocationContext{}, &r.child)
}

func (s *RationalService) CleanupCopyAllocation(r *RationalCopyAllocationReceipt) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if r == nil || r.service != s || !s.Manager.Z.MatchesCopyAllocationInvocation(&r.child) {
		return arena.ErrInvalidSlot
	}
	return s.Manager.Z.CleanupCopyAllocationInvocation(&r.child)
}
