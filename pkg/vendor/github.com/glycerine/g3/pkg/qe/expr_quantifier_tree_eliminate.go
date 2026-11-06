package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::elim_var.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// eliminateExprQETreeVariable installs the solved unit branch. The supplied
// normalizer must use the resulting child variable set.
func eliminateExprQETreeVariable(node *exprQETree, index uint32, formula, definition ast.ASTHandle, variables *exprQEBranchVariables, config Config, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer) (*exprQETree, error) {
	if node == nil || node.manager == nil || variables == nil || variables.manager != node.manager || rw == nil || nnf == nil {
		return nil, ast.ErrASTHandle
	}
	if index >= uint32(len(node.variables)) {
		return nil, arena.ErrBounds
	}
	variable, err := node.manager.RefGet(node.variables[index])
	if err != nil {
		return nil, err
	}
	if err = node.selectVariable(variable, big.NewRat(1, 1)); err != nil {
		return nil, err
	}
	child, err := node.addUnit(config, formula)
	if err != nil {
		return nil, err
	}
	if err = child.addDefinition(variable, definition); err != nil {
		return nil, err
	}
	if err = variables.consume(child); err != nil {
		return nil, err
	}
	if err = normalizeExprQETree(child, rw, nnf); err != nil {
		return nil, err
	}
	return child, nil
}
