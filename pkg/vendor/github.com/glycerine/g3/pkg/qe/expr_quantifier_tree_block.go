package qe

// Source: src/qe/qe.cpp, pop and block_assignment.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// blockExprQETreeIfSolved closes a completed path in the branch solver so the
// search can visit the remaining assignments. An unsolved node is unchanged.
func blockExprQETreeIfSolved(node *exprQETree, solver *smtcore.SMTSolver) (bool, error) {
	if node == nil || node.manager == nil || solver == nil || node.formula == (ast.ASTRef{}) || node.selected != (ast.ASTRef{}) || len(node.children) != 0 {
		return false, ast.ErrASTDomain
	}
	if len(node.variables) != 0 {
		return false, nil
	}
	if err := assertExprQETreeConstraint(node, solver, true); err != nil {
		return false, err
	}
	return true, nil
}
