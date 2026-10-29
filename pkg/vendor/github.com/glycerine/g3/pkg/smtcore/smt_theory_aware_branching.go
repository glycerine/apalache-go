package smtcore

// Source: src/smt/smt_case_split_queue.cpp theory_aware_act_lt and
// theory_aware_branching_queue; smt_context.cpp branching-info dispatch.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uheap "github.com/glycerine/g3/pkg/util/heap"
	"github.com/glycerine/g3/pkg/util/lbool"
	z3rand "github.com/glycerine/g3/pkg/util/random"
)

type smtTheoryAwareBranchingQueue struct {
	state      *SMTInternalizerState
	priorities map[int32]float64
	heap       *uheap.Int
}

func (s *SMTInternalizerState) ensureTheoryAwareBranching() error {
	if err := s.open(); err != nil {
		return err
	}
	if s.theoryAwareBranching != nil {
		return nil
	}
	q := &smtTheoryAwareBranchingQueue{state: s, priorities: make(map[int32]float64)}
	less := func(first, second int32) bool {
		return q.priorities[first]+s.activity[first] > q.priorities[second]+s.activity[second]
	}
	var err error
	q.heap, err = uheap.New(1024, uheap.Config{Budget: s.manager.Budget(), Less: less})
	if err != nil {
		return err
	}
	for variable := int32(0); variable < int32(len(s.boolExpr)); variable++ {
		if err := q.created(variable); err != nil {
			_ = q.heap.Close()
			return err
		}
	}
	s.theoryAwareBranching = q
	return nil
}

func (q *smtTheoryAwareBranchingQueue) created(variable int32) error {
	if variable >= q.heap.Bounds() {
		if err := q.heap.SetBounds(variable + 1); err != nil {
			return err
		}
	}
	return q.heap.Insert(variable)
}

func (q *smtTheoryAwareBranchingQueue) unassigned(variable int32) error {
	if !q.heap.Contains(variable) {
		return q.heap.Insert(variable)
	}
	return nil
}

func (q *smtTheoryAwareBranchingQueue) deleted(variable int32) error {
	if q.heap.Contains(variable) {
		return q.heap.Erase(variable)
	}
	// Source del_var_eh does not erase priority metadata. A reused Boolean
	// slot consequently inherits its previous theory priority.
	return nil
}

func (q *smtTheoryAwareBranchingQueue) activityIncreased(variable int32) error {
	if q.heap.Contains(variable) {
		return q.heap.Decreased(variable)
	}
	return nil
}

func (s *SMTInternalizerState) AddTheoryAwareBranchingInfo(variable int32, priority float64, phase lbool.LBool) error {
	if err := s.open(); err != nil {
		return err
	}
	if variable < 0 || int(variable) >= len(s.activity) {
		return ast.ErrASTDomain
	}
	if s.caseSplitStrategy != 6 {
		return nil // Other native queue implementations use the no-op callback.
	}
	if err := s.ensureTheoryAwareBranching(); err != nil {
		return err
	}
	if err := s.traceTheoryPriority(variable, priority); err != nil {
		return err
	}
	q := s.theoryAwareBranching
	q.priorities[variable] = priority
	// Native phase lookup occurs only after next=null_bool_var; successful
	// choices return earlier. No valid selected variable uses that map.
	_ = phase
	if q.heap.Contains(variable) {
		if priority > 0 {
			return q.heap.Decreased(variable)
		}
		return q.heap.Increased(variable)
	}
	return nil
}

func (q *smtTheoryAwareBranchingQueue) next(frequency float64) (int32, error) {
	threshold := int(frequency * float64(z3rand.Max))
	if err := smtCaseSplitAssert(1167, "threshold >= 0", func() bool { return threshold >= 0 }); err != nil {
		return SMTNullBoolVar, err
	}
	if int(q.state.random.Next()) < threshold {
		if err := smtCaseSplitAssert(1169, "m_context.get_num_b_internalized() > 0", func() bool { return len(q.state.boolExpr) > 0 }); err != nil {
			return SMTNullBoolVar, err
		}
		if len(q.state.boolExpr) == 0 {
			return SMTNullBoolVar, ast.ErrASTDomain
		}
		candidate := int32(q.state.random.Next() % uint32(len(q.state.boolExpr)))
		value, err := q.state.Assignment(NewSMTLiteral(candidate, false))
		if err != nil {
			return SMTNullBoolVar, err
		}
		if err = q.state.traceRandomSplit(1171, candidate, value); err != nil {
			return SMTNullBoolVar, err
		}
		if value == lbool.Undef {
			return candidate, nil // Random selection leaves the heap entry present.
		}
	}
	for !q.heap.Empty() {
		variable, err := q.heap.EraseMin()
		if err != nil {
			return SMTNullBoolVar, err
		}
		value, err := q.state.Assignment(NewSMTLiteral(variable, false))
		if err != nil {
			return SMTNullBoolVar, err
		}
		if value == lbool.Undef {
			return variable, nil
		}
	}
	return SMTNullBoolVar, nil
}

func (q *smtTheoryAwareBranchingQueue) display(out io.Writer) error {
	if q == nil || q.state == nil || out == nil {
		return ast.ErrASTHandle
	}
	if err := q.state.open(); err != nil {
		return err
	}
	values, err := q.heap.AppendValues(nil)
	if err != nil {
		return err
	}
	first := true
	for _, variable := range values {
		assignment, err := q.state.Assignment(NewSMTLiteral(variable, false))
		if err != nil {
			return err
		}
		if assignment != lbool.Undef {
			continue
		}
		if first {
			if _, err := io.WriteString(out, "remaining case-splits:\n"); err != nil {
				return err
			}
			first = false
		}
		id, err := q.state.manager.GetASTID(q.state.boolExpr[variable])
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "#%d ", id); err != nil {
			return err
		}
	}
	if !first {
		_, err := io.WriteString(out, "\n")
		return err
	}
	return nil
}
