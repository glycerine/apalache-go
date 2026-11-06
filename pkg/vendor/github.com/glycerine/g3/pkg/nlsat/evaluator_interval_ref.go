package nlsat

// Concrete retained interval_set_ref ownership for evaluator locals.
// Source: src/util/obj_ref.h:79-103; src/nlsat/nlsat_evaluator.cpp.

import "github.com/glycerine/g3/pkg/math/polynomial"

type evaluatorIntervalRef struct {
	manager         *IntervalSetManager
	value, accepted *IntervalSet
	pending         bool
}

func (r *evaluatorIntervalRef) drop(dst **IntervalSet) error {
	s := *dst
	if s == nil {
		return nil
	}
	err := r.manager.DecRef(s)
	// A last decrement is consumed before numeral/allocator retirement can
	// stop. That retirement is now retained by the actual interval manager.
	if err == nil || s.deleting || s.closed {
		*dst = nil
	}
	return err
}
func (r *evaluatorIntervalRef) assign(s *IntervalSet) error {
	if r.pending {
		return polynomial.ErrAlgebraicPending
	}
	if s != nil {
		if err := r.manager.IncRef(s); err != nil {
			return err
		}
	}
	r.accepted = s
	r.pending = true
	if err := r.drop(&r.value); err != nil {
		return err
	}
	r.value = r.accepted
	r.accepted = nil
	r.pending = false
	return nil
}
func (r *evaluatorIntervalRef) reset() error {
	if r.pending {
		if err := r.drop(&r.accepted); err != nil {
			return err
		}
		r.pending = false
	}
	return r.drop(&r.value)
}
