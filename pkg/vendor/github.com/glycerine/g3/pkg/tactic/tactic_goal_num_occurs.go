package tactic

// Source: src/tactic/goal_num_occurs.cpp; src/tactic/goal_num_occurs.h.

import "github.com/glycerine/g3/pkg/ast"

// GoalNumOccurs applies the AST occurrence counter to all formulas in a goal
// with one shared visited set, matching goal_num_occurs::operator().
type GoalNumOccurs struct {
	counter *ast.NumOccurs
}

func NewGoalNumOccurs(m *ast.ASTManager, ignoreRefCountOne, ignoreQuantifiers bool) (*GoalNumOccurs, error) {
	counter, err := ast.NewNumOccurs(m, ignoreRefCountOne, ignoreQuantifiers)
	if err != nil {
		return nil, err
	}
	return &GoalNumOccurs{counter: counter}, nil
}

func (n *GoalNumOccurs) Reset() {
	if n != nil && n.counter != nil {
		n.counter.Reset()
	}
}

func (n *GoalNumOccurs) Count(g *Goal) error {
	if n == nil || n.counter == nil {
		return ast.ErrASTHandle
	}
	if err := g.open(); err != nil {
		return err
	}
	return n.counter.CountAll(g.forms)
}

func (n *GoalNumOccurs) GetNumOccurrences(h ast.ASTHandle) uint32 {
	if n == nil || n.counter == nil {
		return 0
	}
	return n.counter.GetNumOccurrences(h)
}
