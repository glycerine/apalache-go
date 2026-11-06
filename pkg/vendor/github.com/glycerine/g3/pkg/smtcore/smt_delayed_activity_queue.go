package smtcore

// Source: src/smt/smt_case_split_queue.cpp:150-217 dact_case_split_queue.

import (
	"errors"

	uheap "github.com/glycerine/g3/pkg/util/heap"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *SMTInternalizerState) ensureDelayedActivityQueue() error {
	if err := s.open(); err != nil {
		return err
	}
	if s.delayedActivity != nil {
		return nil
	}
	hadPrimary := s.basicActivity != nil
	if err := s.ensureBasicActivityQueue(); err != nil {
		return err
	}
	h, err := uheap.New(1024, uheap.Config{Budget: s.manager.Budget(), Less: func(a, b int32) bool { return s.activity[a] > s.activity[b] }})
	if err != nil {
		if !hadPrimary {
			closeErr := s.basicActivity.Close()
			if closeErr == nil {
				s.basicActivity = nil
			}
			err = errors.Join(err, closeErr)
		}
		return err
	}
	s.delayedActivity = h
	return nil
}

func (q *SMTActivityCaseSplitQueue) nextDelayedActivity() (int32, lbool.LBool, error) {
	if err := q.state.ensureDelayedActivityQueue(); err != nil {
		return SMTNullBoolVar, lbool.Undef, err
	}
	v, phase, err := q.nextBasicActivity()
	if err != nil || v != SMTNullBoolVar {
		return v, phase, err
	}
	// Source swaps both heap storage vectors and inverse positions. The
	// second drain does not make another random-selection draw.
	if err = q.state.basicActivity.Swap(q.state.delayedActivity); err != nil {
		return SMTNullBoolVar, lbool.Undef, err
	}
	if err = smtCaseSplitAssert(208, "m_delayed_queue.empty()", func() bool { return q.state.delayedActivity.Empty() }); err != nil {
		return SMTNullBoolVar, lbool.Undef, err
	}
	return q.state.popBasicActivityHeap()
}
