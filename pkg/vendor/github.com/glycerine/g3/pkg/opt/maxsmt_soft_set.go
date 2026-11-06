package opt

// Source: src/opt/maxsmt.cpp maxsmt::add and maxsmt_solver_base ownership.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// weightedSoftSet owns normalized, unique soft formulas until Release transfers
// their references into ArithmeticSolver's objective.
type weightedSoftSet struct {
	manager *ast.ASTManager
	terms   []ast.ASTRef
	weights []*big.Rat
	index   map[ast.ASTHandle]int
	offset  big.Rat
	closed  bool
}

func newWeightedSoftSet(manager *ast.ASTManager) (*weightedSoftSet, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &weightedSoftSet{manager: manager, index: make(map[ast.ASTHandle]int)}, nil
}

func (s *weightedSoftSet) Len() int {
	if s == nil || s.closed {
		return 0
	}
	return len(s.terms)
}
