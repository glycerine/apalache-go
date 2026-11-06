package smtcore

// Source: src/smt/smt_case_split_queue.cpp:298-483 rel_case_split_queue.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtRelevanceFIFOScope struct {
	firstLimit, secondLimit int
	firstHead, secondHead   int
}

type smtRelevanceFIFO struct {
	first, second         []ast.ASTHandle
	firstHead, secondHead int
	// Source int m_bs_num_bool_vars is initialized/reset with UINT_MAX (-1).
	baseBooleanLimit int32
}

func (f *smtRelevanceFIFO) snapshot() smtRelevanceFIFOScope {
	return smtRelevanceFIFOScope{len(f.first), len(f.second), f.firstHead, f.secondHead}
}

func (f *smtRelevanceFIFO) restore(scope smtRelevanceFIFOScope, assertHead bool) error {
	if scope.firstLimit < 0 || scope.firstLimit > len(f.first) || scope.secondLimit < 0 || scope.secondLimit > len(f.second) ||
		scope.firstHead < 0 || scope.secondHead < 0 || scope.secondHead > scope.secondLimit {
		return ast.ErrASTDomain
	}
	clear(f.first[scope.firstLimit:])
	clear(f.second[scope.secondLimit:])
	f.first, f.second = f.first[:scope.firstLimit], f.second[:scope.secondLimit]
	f.firstHead, f.secondHead = scope.firstHead, scope.secondHead
	if assertHead {
		if err := smtCaseSplitAssert(399, "m_head <= m_queue.size()", func() bool { return f.firstHead <= len(f.first) }); err != nil {
			return err
		}
	}
	if f.firstHead > len(f.first) {
		return ast.ErrASTDomain
	}
	return nil
}

// Called at the actual relevance event, before theory relevance callbacks.
func (s *SMTInternalizerState) admitRelevanceCaseSplit(expression ast.ASTHandle) error {
	boolean, err := s.isBoolean(expression)
	if err != nil || !boolean {
		return err
	}
	op, basic, err := s.basicOperation(expression)
	if err != nil {
		return err
	}
	isOr, isAnd := basic && op == ast.ASTOpOr, basic && op == ast.ASTOpAnd
	variable, internalized := s.BoolVar(expression)
	if !internalized {
		variable = SMTNullBoolVar
		if !isOr {
			return nil
		}
	} else {
		if err := s.assertCaseSplitAdmissionVar(variable); err != nil {
			return err
		}
		value, err := s.Assignment(NewSMTLiteral(variable, false))
		if err != nil {
			return err
		}
		if value != lbool.Undef && !(isOr && value == lbool.True) && !(isAnd && value == lbool.False) {
			return nil
		}
	}
	if !internalized && s.searching {
		if err := s.assertCaseSplitAdmissionOr(isOr); err != nil {
			return err
		}
	}
	if err := s.admitRelevanceGoal(expression, variable, internalized); err != nil {
		return err
	}
	if (!internalized && !s.searching) || (internalized && variable < s.relevanceFIFO.baseBooleanLimit) {
		s.relevanceActivity.entries = append(s.relevanceActivity.entries, expression)
	}
	f := &s.relevanceFIFO
	if !internalized && s.searching || variable >= f.baseBooleanLimit {
		f.second = append(f.second, expression)
	} else {
		f.first = append(f.first, expression)
	}
	return nil
}

func (q *SMTActivityCaseSplitQueue) nextRelevanceFIFO() (int32, lbool.LBool, error) {
	f := &q.state.relevanceFIFO
	for _, second := range []bool{false, true} {
		queue, head := f.first, &f.firstHead
		if second {
			queue, head = f.second, &f.secondHead
		}
		// Native captures queue.size once; successful candidates retain head.
		for limit := len(queue); *head < limit; (*head)++ {
			variable, phase, usable, err := q.relevantCandidate(queue[*head])
			if err != nil || usable {
				return variable, phase, err
			}
		}
	}
	return SMTNullBoolVar, lbool.Undef, nil
}

// Source profile3 display includes consumed entries and omits a HEAD marker
// when the cursor is at the end; it does not compact or consume the queues.
func (s *SMTInternalizerState) DisplayRelevanceCaseSplitQueue(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	f := &s.relevanceFIFO
	if len(f.first) == 0 && len(f.second) == 0 {
		return nil
	}
	if err := smtWriteTheoryText(out, "case-splits:\n"); err != nil {
		return err
	}
	for index, queue := range [][]ast.ASTHandle{f.first, f.second} {
		if len(queue) == 0 {
			continue
		}
		head := f.firstHead
		if index == 1 {
			head = f.secondHead
		}
		for i, expression := range queue {
			if i == head {
				if err := smtWriteTheoryText(out, fmt.Sprintf("[HEAD%d]=> ", index+1)); err != nil {
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
		if err := smtWriteTheoryText(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}
