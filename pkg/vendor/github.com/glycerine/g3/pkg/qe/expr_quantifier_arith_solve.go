package qe

// Source: src/qe/qe.cpp, publishing an arithmetic solver elimination.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// applyExprQEArithmeticResult borrows the arithmetic result and publishes its
// definition, formula and fresh variables as a solved unit branch.
func applyExprQEArithmeticResult(node *exprQETree, variables *exprQEBranchVariables, context *SolverContext, result *ArithmeticEquationElimination, config Config, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer) (*exprQETree, error) {
	if node == nil || node.manager == nil || variables == nil || context == nil || result == nil || rw == nil || nnf == nil {
		return nil, ast.ErrASTHandle
	}
	m := node.manager
	variable, err := context.Variable(result.VariableIndex)
	if err != nil {
		return nil, err
	}
	index, err := node.variableIndex(variable)
	if err != nil {
		return nil, err
	}
	if index < 0 {
		return nil, ast.ErrASTDomain
	}
	for _, ref := range result.Fresh {
		fresh, e := m.RefGet(ref)
		if e != nil {
			return nil, e
		}
		if e = variables.add(fresh); e != nil {
			return nil, e
		}
	}
	formula, err := m.RefGet(result.Formula)
	if err != nil {
		return nil, err
	}
	definition, err := m.RefGet(result.Definition)
	if err != nil {
		return nil, err
	}
	return eliminateExprQETreeVariable(node, uint32(index), formula, definition, variables, config, rw, nnf)
}
