package qe

// Source: src/qe/qe.cpp, constrain_assignment.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

// chooseExprQETreeBranch selects a supported variable and publishes the
// corresponding bound. Unsupported variables are appended to freeVariables.
func chooseExprQETreeBranch(node *exprQETree, variables *exprQEBranchVariables, context *SolverContext, solver *smtcore.SMTSolver, freeVariables *[]ast.ASTRef) (bool, error) {
	if node == nil || node.atoms == nil || solver == nil {
		return false, ast.ErrASTHandle
	}
	variable, count, found, err := selectExprQEVariable(node, variables, context, node.atoms, freeVariables)
	if err != nil || !found {
		return false, err
	}
	if err = constrainExprQEBranch(node, variables, solver, variable, &count); err != nil {
		return false, err
	}
	return true, nil
}
