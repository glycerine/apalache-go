package smtcore

// Source: src/smt/smt_case_split_queue.cpp:486-675 rel_act_case_split_queue.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uheap "github.com/glycerine/g3/pkg/util/heap"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtRelevanceActivityQueue struct {
	entries []ast.ASTHandle
	head    int
	delayed *uheap.Int
}

func (s *SMTInternalizerState) ensureRelevanceActivityQueue() error {
	if err := s.open(); err != nil {
		return err
	}
	if s.relevanceActivity.delayed != nil {
		return nil
	}
	h, err := uheap.New(1024, uheap.Config{Budget: s.manager.Budget(), Less: func(a, b int32) bool { return s.activity[a] > s.activity[b] }})
	if err != nil {
		return err
	}
	s.relevanceActivity.delayed = h
	return nil
}

func (s *SMTInternalizerState) relevanceActivityCreated(variable int32) error {
	h := s.relevanceActivity.delayed
	if h == nil || !s.searching {
		return nil
	}
	if err := smtCaseSplitAssert(514, "v >= m_bs_num_bool_vars", func() bool { return variable >= s.relevanceFIFO.baseBooleanLimit }); err != nil {
		return err
	}
	if err := h.Reserve(variable + 1); err != nil {
		return err
	}
	return h.Insert(variable)
}

func (s *SMTInternalizerState) relevanceActivityUnassigned(variable int32) error {
	h := s.relevanceActivity.delayed
	if h == nil || variable < s.relevanceFIFO.baseBooleanLimit || h.Contains(variable) {
		return nil
	}
	if err := h.Reserve(variable + 1); err != nil {
		return err
	}
	return h.Insert(variable)
}

func (s *SMTInternalizerState) relevanceActivityDeleted(variable int32) error {
	h := s.relevanceActivity.delayed
	if h != nil && variable >= s.relevanceFIFO.baseBooleanLimit && h.Contains(variable) {
		return h.Erase(variable)
	}
	return nil
}

func (q *SMTActivityCaseSplitQueue) nextRelevanceActivity() (int32, lbool.LBool, error) {
	if q.state.relevanceActivity.delayed == nil {
		return SMTNullBoolVar, lbool.Undef, ast.ErrASTHandle
	}
	if next, err := q.randomVariable(0.02); err != nil || next != SMTNullBoolVar {
		return next, lbool.Undef, err
	}
	f := &q.state.relevanceActivity
	for limit := len(f.entries); f.head < limit; f.head++ {
		variable, phase, usable, err := q.relevantCandidate(f.entries[f.head])
		if err != nil || usable {
			return variable, phase, err
		}
	}
	for !f.delayed.Empty() {
		variable, err := f.delayed.EraseMin()
		if err != nil {
			return SMTNullBoolVar, lbool.Undef, err
		}
		value, err := q.state.Assignment(NewSMTLiteral(variable, false))
		if err != nil {
			return SMTNullBoolVar, lbool.Undef, err
		}
		if value == lbool.Undef {
			return variable, lbool.Undef, nil
		}
	}
	return SMTNullBoolVar, lbool.Undef, nil
}

func (s *SMTInternalizerState) DisplayRelevanceActivityQueue(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	f := &s.relevanceActivity
	if len(f.entries) == 0 {
		return nil
	}
	if err := smtWriteTheoryText(out, "case-splits:\n"); err != nil {
		return err
	}
	for i, expression := range f.entries {
		if i == f.head {
			if err := smtWriteTheoryText(out, "[HEAD]=> "); err != nil {
				return err
			}
		}
		id, err := s.manager.GetASTID(expression)
		if err != nil {
			return err
		}
		if err = smtWriteTheoryText(out, fmt.Sprintf("#%d ", id)); err != nil {
			return err
		}
	}
	return smtWriteTheoryText(out, "\n")
}
