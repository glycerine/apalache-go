package smtcore

// Source: src/smt/smt_case_split_queue.cpp:678-1119 rel_goal_case_split_queue.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uheap "github.com/glycerine/g3/pkg/util/heap"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type smtRelevanceGoalEntry struct {
	expression  ast.ASTHandle
	generation  uint32
	lastDecided int32
}

type smtRelevanceGoalScope struct {
	firstLimit, secondLimit, head int
	level                         uint32
	goal                          ast.ASTHandle
}

type smtRelevanceGoalQueue struct {
	first   []ast.ASTHandle
	head    int
	second  []smtRelevanceGoalEntry
	heap    *uheap.Int
	current ast.ASTHandle
}

func (s *SMTInternalizerState) ensureRelevanceGoalQueue() error {
	if err := s.open(); err != nil {
		return err
	}
	f := &s.relevanceGoal
	if f.heap != nil {
		return nil
	}
	h, err := uheap.New(0, uheap.Config{Budget: s.manager.Budget(), Less: func(a, b int32) bool {
		x, y := f.second[a], f.second[b]
		return x.generation < y.generation || x.generation == y.generation && a < b
	}})
	if err != nil {
		return err
	}
	f.heap = h
	// Native start_gen is zero; no live writer increments current_generation.
	// Its commented decrement and >=100 generation-lowering arm are dead.
	s.generation = 0
	return nil
}

func (s *SMTInternalizerState) admitRelevanceGoal(expression ast.ASTHandle, variable int32, internalized bool) error {
	f := &s.relevanceGoal
	if f.heap == nil {
		return nil
	}
	if !(!internalized && s.searching) && variable < s.relevanceFIFO.baseBooleanLimit {
		f.first = append(f.first, expression)
		return nil
	}
	generation, err := s.expressionGeneration(expression)
	if err != nil {
		return err
	}
	index := int32(len(f.second))
	f.second = append(f.second, smtRelevanceGoalEntry{expression, generation, -1})
	if err = f.heap.Reserve(index + 1); err != nil {
		return err
	}
	return f.heap.Insert(index)
}

func (f *smtRelevanceGoalQueue) snapshot(level uint32) smtRelevanceGoalScope {
	return smtRelevanceGoalScope{len(f.first), len(f.second), f.head, level, f.current}
}

func (f *smtRelevanceGoalQueue) restore(scope smtRelevanceGoalScope, assertHead bool) error {
	if scope.firstLimit < 0 || scope.firstLimit > len(f.first) || scope.secondLimit < 0 || scope.secondLimit > len(f.second) || scope.head < 0 {
		return ast.ErrASTDomain
	}
	clear(f.first[scope.firstLimit:])
	f.first = f.first[:scope.firstLimit]
	f.head, f.current = scope.head, scope.goal
	if f.heap != nil {
		for i := scope.secondLimit; i < len(f.second); i++ {
			if err := smtCaseSplitAssert(841, "(m_queue2[i].m_last_decided == -1) == m_priority_queue2.contains(i)", func() bool { return (f.second[i].lastDecided == -1) == f.heap.Contains(int32(i)) }); err != nil {
				return err
			}
			if f.heap.Contains(int32(i)) {
				if err := f.heap.Erase(int32(i)); err != nil {
					return err
				}
			}
		}
		for i := 0; i < scope.secondLimit; i++ {
			if f.second[i].lastDecided > int32(scope.level) {
				if err := smtCaseSplitAssert(850, "!m_priority_queue2.contains(i)", func() bool { return !f.heap.Contains(int32(i)) }); err != nil {
					return err
				}
				if err := f.heap.Insert(int32(i)); err != nil {
					return err
				}
				f.second[i].lastDecided = -1
			}
		}
	}
	clear(f.second[scope.secondLimit:])
	f.second = f.second[:scope.secondLimit]
	if assertHead {
		if err := smtCaseSplitAssert(859, "m_head <= m_queue.size()", func() bool { return f.head <= len(f.first) }); err != nil {
			return err
		}
	}
	if f.head > len(f.first) {
		return ast.ErrASTDomain
	}
	return nil
}

func (q *SMTActivityCaseSplitQueue) nextRelevanceGoal() (int32, lbool.LBool, error) {
	f := &q.state.relevanceGoal
	if f.heap == nil {
		return SMTNullBoolVar, lbool.Undef, ast.ErrASTHandle
	}
	for limit := len(f.first); f.head < limit; f.head++ {
		v, phase, usable, err := q.relevantCandidate(f.first[f.head])
		if err != nil || usable {
			return v, phase, err
		}
	}
	for !f.heap.Empty() {
		index, err := f.heap.EraseMin()
		if err != nil {
			return SMTNullBoolVar, lbool.Undef, err
		}
		if err = q.state.emitCaseSplitTrace("case_split", "next_case_split", 915, func(out io.Writer) error {
			return smtWriteTheoryText(out, fmt.Sprintf("q %d idx %d\n", len(f.second), index))
		}); err != nil {
			return SMTNullBoolVar, lbool.Undef, err
		}
		if err = smtCaseSplitAssert(916, "idx < m_queue2.size()", func() bool { return index >= 0 && int(index) < len(f.second) }); err != nil {
			return SMTNullBoolVar, lbool.Undef, err
		}
		if index < 0 || int(index) >= len(f.second) {
			return SMTNullBoolVar, lbool.Undef, ast.ErrASTDomain
		}
		if err = smtCaseSplitAssert(918, "e.m_last_decided == -1", func() bool { return f.second[index].lastDecided == -1 }); err != nil {
			return SMTNullBoolVar, lbool.Undef, err
		}
		f.second[index].lastDecided = int32(q.state.scopeLevel)
		v, phase, usable, err := q.relevantCandidate(f.second[index].expression)
		if err != nil {
			return SMTNullBoolVar, lbool.Undef, err
		}
		if usable {
			if err = f.heap.Insert(index); err != nil {
				return SMTNullBoolVar, lbool.Undef, err
			}
			f.second[index].lastDecided = -1
			return v, phase, nil
		}
	}
	return SMTNullBoolVar, lbool.Undef, nil
}

func (s *SMTInternalizerState) relevanceGoalAssigned(literal SMTLiteral) error {
	f := &s.relevanceGoal
	if f.heap == nil {
		return nil
	}
	expression := s.boolExpr[literal.Var()]
	if expression == f.current {
		return nil
	}
	op, basic, err := s.basicOperation(expression)
	if err != nil {
		return err
	}
	sign := literal.Sign()
	if !basic || !((op == ast.ASTOpAnd && !sign) || (op == ast.ASTOpOr && sign)) {
		return nil
	}
	count, err := s.manager.GetNumArgs(expression)
	if err != nil || count != 2 {
		return err
	}
	label, err := s.manager.GetArg(expression, 1)
	if err != nil {
		return err
	}
	labelOp, labelBasic, err := s.basicOperation(label)
	if err != nil {
		return err
	}
	if labelBasic && labelOp == ast.ASTOpNot {
		sign = !sign
		label, err = s.manager.GetArg(label, 0)
		if err != nil {
			return err
		}
	}
	if sign {
		return nil
	}
	var names []usymbol.Symbol
	matched, err := s.manager.IsLabelLit(label, &names)
	if err != nil {
		return err
	}
	if matched {
		if err = s.traceFoundGoal(expression); err != nil {
			return err
		}
		f.current = expression
	}
	return nil
}

func (s *SMTInternalizerState) DisplayRelevanceGoalQueue(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	f := &s.relevanceGoal
	if len(f.first) == 0 && len(f.second) == 0 {
		return nil
	}
	if err := smtWriteTheoryText(out, "case-splits:\n"); err != nil {
		return err
	}
	// Source deliberately comments out secondary queue rendering.
	if len(f.first) == 0 {
		return nil
	}
	for i, expression := range f.first {
		if i == f.head {
			if err := smtWriteTheoryText(out, "[HEAD1]=> "); err != nil {
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
