package polynomial

import "errors"

// Actual declarations at factorization.cpp:434-440, in source order.
type upolyBerlekampSplit struct {
	gcd, div, vector               upolyVectorState
	declared, ownStarted, ownEnded bool
	sourceRemaining, ownRemaining  uint8
	sourceIndex, ownIndex          uint32
	cause                          error
}

func (s *upolyBerlekampSplit) at(i uint8) *upolyVectorState {
	switch i {
	case 1:
		return &s.gcd
	case 2:
		return &s.div
	case 3:
		return &s.vector
	}
	return nil
}
func (q *upolyBerlekampMatrix) traceSourceBlocker() error {
	if e := q.outcomeFactors.sourceBlocker(); e != nil {
		return e
	}
	if q.m.displayReceiptPending(&q.entryTrace) {
		if e := q.m.displayReceiptSourceCause(&q.entryTrace); e != nil {
			return e
		}
		return ErrUPolynomialCleanupPending
	}
	return nil
}
func (q *upolyBerlekampMatrix) finishSplitSource(body error) error {
	s := &q.split
	if !s.declared || s.sourceRemaining == 0 {
		return body
	}
	if s.cause == nil {
		s.cause = q.traceSourceBlocker()
	}
	if s.cause == nil {
		s.cause = q.numeric.sourceCause()
	}
	if s.cause != nil {
		q.retainSplitStop()
		return errors.Join(body, s.cause)
	}
	q.destroying = true
	defer func() { q.destroying = false }()
	var borrow upolyFactorTraceBorrow
	if e := borrow.bindManagers([3]*UPolynomialManager{q.m}, q.m.modular.BorrowTraceTags()); e != nil {
		s.cause = e
		q.retainSplitStop()
		return errors.Join(body, e)
	}
	defer borrow.unbind()
	for s.sourceRemaining != 0 {
		v := s.at(s.sourceRemaining)
		for s.sourceIndex < v.size {
			if _, e := upolynomialSourceNumeralDel(q.m.config.Integer, &q.m.values(v)[s.sourceIndex]); e != nil {
				s.cause = e
				q.retainSplitStop()
				return errors.Join(body, e)
			}
			s.sourceIndex++
		}
		q.m.freeBacking(v)
		s.sourceRemaining--
		s.sourceIndex = 0
	}
	return body
}
func (q *upolyBerlekampMatrix) retainSplitStop() {
	if q.closeCause == nil {
		q.closeCause = q.split.cause
	}
	if q.m.pendingBerlekamp == nil || q.m.pendingBerlekamp == q {
		q.m.pendingBerlekamp = q
	}
}
func (q *upolyBerlekampMatrix) cleanupSplitOwned() error {
	s := &q.split
	if !s.declared || s.ownEnded {
		return nil
	}
	if !s.ownStarted {
		s.ownStarted = true
		s.ownRemaining = s.sourceRemaining
		s.ownIndex = s.sourceIndex
	}
	for s.ownRemaining != 0 {
		v := s.at(s.ownRemaining)
		for s.ownIndex < v.size {
			if e := q.m.config.Integer.CleanupRawNumeral(&q.m.values(v)[s.ownIndex]); e != nil {
				return e
			}
			s.ownIndex++
		}
		q.m.freeBacking(v)
		s.ownRemaining--
		s.ownIndex = 0
	}
	s.ownEnded = true
	return nil
}
