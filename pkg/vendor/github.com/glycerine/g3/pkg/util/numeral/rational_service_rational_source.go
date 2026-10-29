package numeral

// Source: src/util/rational.h:52; src/util/mpq.h set(mpq const&).

import (
	"sync/atomic"

	umemory "github.com/glycerine/g3/pkg/util/memory"
)

// NewFromRationalOwnerWithInvocation constructs rational(mpq const&) from an
// explicitly borrowed raw owner. NUM is copied before DEN, without reduction
// or an intermediate pair assignment. A failed constructor's actual prefix
// belongs to call; the source and a successfully returned value do not.
func (s *RationalService) NewFromRationalOwnerWithInvocation(source *RationalManager, value *Rational, call *RationalServiceInvocation) (out OwnedRational, e error) {
	if e = s.lock(); e != nil {
		return
	}
	defer s.mu.Unlock()
	if source == nil || source.self != source {
		return out, umemory.ErrRuntimeOwner
	}
	if e = source.check(value); e != nil {
		return
	}
	if source.Z.rationalServiceRead != nil {
		return out, ErrNumericPending
	}
	if e = s.prepareLexicalInvocation(call); e != nil {
		return
	}
	defer func() { e = s.finishLexicalInvocation(call, e) }()
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e = s.beginLexical(); e != nil {
		return
	}
	defer s.endLexicalRational(&out, &e)
	call.borrowedInteger = source.Z
	source.Z.rationalServiceRead = call
	defer func() {
		if source.Z.rationalServiceRead == call {
			source.Z.rationalServiceRead = nil
		}
	}()
	result := NewRational()
	complete := false
	defer s.endLexicalLocal(&result, &complete)
	context := call.allocationContext()
	if e = s.Manager.Z.copyFromOwnerWithAllocationContext(&result.Num, source.Z, &value.Num, nil, context); e != nil {
		return
	}
	if e = s.Manager.Z.copyFromOwnerWithAllocationContext(&result.Den, source.Z, &value.Den, nil, context); e != nil {
		return
	}
	complete = true
	return s.facadeTakeLocked(&result)
}

// SourceDelValue is the literal rational destructor, separate from the owning
// CleanupValue continuation. A stopped NUM/DEN deletion retains the token and
// its actual remaining fields; owning cleanup must not replay this destructor.
func (s *RationalService) SourceDelValue(value *OwnedRational) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if value == nil || value.ownerID == 0 || value.ownerID != s.ownerID {
		return ErrRationalServiceOwner
	}
	atomic.StoreUint32(&s.sourceRuntimeExecuting, 1)
	defer atomic.StoreUint32(&s.sourceRuntimeExecuting, 0)
	if e := SourceRationalDel(s.Manager, &value.Raw); e != nil {
		return e
	}
	*value = OwnedRational{}
	return nil
}
