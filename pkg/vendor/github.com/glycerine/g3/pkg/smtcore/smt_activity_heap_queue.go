package smtcore

// Source: src/smt/smt_case_split_queue.cpp:64-148 act_case_split_queue.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uheap "github.com/glycerine/g3/pkg/util/heap"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *SMTInternalizerState) ensureBasicActivityQueue() error {
	if err := s.open(); err != nil {
		return err
	}
	if s.basicActivity != nil {
		return nil
	}
	h, err := uheap.New(1024, uheap.Config{Budget: s.manager.Budget(), Less: func(a, b int32) bool { return s.activity[a] > s.activity[b] }})
	if err != nil {
		return err
	}
	// Source creates the reserved true variable before ordinary variables.
	// Standalone Go selectors can be configured after internalization; seed
	// the existing variables in their original creation order exactly once.
	for v := int32(0); int(v) < len(s.boolExpr); v++ {
		if err = h.Reserve(v + 1); err == nil {
			err = h.Insert(v)
		}
		if err != nil {
			_ = h.Close()
			return err
		}
	}
	s.basicActivity = h
	return nil
}

func (s *SMTInternalizerState) basicActivityCreated(v int32) error {
	if s.basicActivity == nil {
		return nil
	}
	if err := s.basicActivity.Reserve(v + 1); err != nil {
		return err
	}
	if s.delayedActivity != nil {
		if err := s.delayedActivity.Reserve(v + 1); err != nil {
			return err
		}

		if err := smtCaseSplitAssert(173, "!m_delayed_queue.contains(v)", func() bool { return !s.delayedActivity.Contains(v) }); err != nil {
			return err
		}
		if err := smtCaseSplitAssert(174, "!m_queue.contains(v)", func() bool { return !s.basicActivity.Contains(v) }); err != nil {
			return err
		}
		if s.searching {
			return s.delayedActivity.Insert(v)
		}
	} else if err := smtCaseSplitAssert(85, "!m_queue.contains(v)", func() bool { return !s.basicActivity.Contains(v) }); err != nil {
		return err
	}
	return s.basicActivity.Insert(v)
}

func (s *SMTInternalizerState) basicActivityUnassigned(v int32) error {
	if s.basicActivity != nil && !s.basicActivity.Contains(v) {
		return s.basicActivity.Insert(v)
	}
	return nil
}

func (s *SMTInternalizerState) basicActivityDeleted(v int32) error {
	if s.basicActivity != nil && s.basicActivity.Contains(v) {
		if err := s.basicActivity.Erase(v); err != nil {
			return err
		}
	}
	if s.delayedActivity != nil && s.delayedActivity.Contains(v) {
		return s.delayedActivity.Erase(v)
	}
	return nil
}

func (q *SMTActivityCaseSplitQueue) nextBasicActivity() (int32, lbool.LBool, error) {
	h := q.state.basicActivity
	if h == nil {
		if err := q.state.ensureBasicActivityQueue(); err != nil {
			return SMTNullBoolVar, lbool.Undef, err
		}
		h = q.state.basicActivity
	}
	if v, err := q.randomVariable(q.randomVariableFrequency); err != nil || v != SMTNullBoolVar {
		return v, lbool.Undef, err
	}
	return q.state.popBasicActivityHeap()
}

func (s *SMTInternalizerState) popBasicActivityHeap() (int32, lbool.LBool, error) {
	h := s.basicActivity
	for !h.Empty() {
		v, err := h.EraseMin()
		if err != nil {
			return SMTNullBoolVar, lbool.Undef, err
		}
		value, err := s.Assignment(NewSMTLiteral(v, false))
		if err != nil {
			return SMTNullBoolVar, lbool.Undef, err
		}
		if value == lbool.Undef {
			return v, lbool.Undef, nil
		}
	}
	return SMTNullBoolVar, lbool.Undef, nil
}

func (s *SMTInternalizerState) DisplayBasicActivityQueue(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if s.basicActivity == nil {
		return nil
	}
	values, err := s.basicActivity.AppendValues(nil)
	if err != nil {
		return err
	}
	first := true
	for _, v := range values {
		value, err := s.Assignment(NewSMTLiteral(v, false))
		if err != nil {
			return err
		}
		if value != lbool.Undef {
			continue
		}
		if first {
			if err = smtWriteTheoryText(out, "remaining case-splits:\n"); err != nil {
				return err
			}
			first = false
		}
		id, err := s.manager.GetASTID(s.boolExpr[v])
		if err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, fmt.Sprintf("#%d ", id)); err != nil {
			return err
		}
	}
	if !first {
		return smtWriteTheoryText(out, "\n")
	}
	return nil
}

// Source cact_case_split_queue inherits the ordinary consuming heap.
func (s *SMTInternalizerState) ensureCachedActivityQueue() error {
	if err := s.ensureBasicActivityQueue(); err != nil {
		return err
	}
	s.cachedActivity = true
	return nil
}
