package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::normalize theory and NNF passes.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type exprQETreeSimplifier interface {
	Simplify(ast.ASTRef, *rewriter.TheoryRewriter) (bool, error)
}

func normalizeExprQETree(node *exprQETree, rw *rewriter.TheoryRewriter, nnf *QENNFNormalizer) (err error) {
	if node == nil || node.manager == nil || rw == nil || nnf == nil || node.formula == (ast.ASTRef{}) {
		return ast.ErrASTHandle
	}
	m := node.manager
	root, err := m.RefGet(node.formula)
	if err != nil {
		return err
	}
	if err = rw.Rewrite(root, node.formula); err != nil {
		return err
	}
	for _, simplifier := range node.simplifiers {
		for {
			changed, cause := simplifier.Simplify(node.formula, rw)
			if cause != nil {
				return cause
			}
			if !changed {
				break
			}
		}
	}
	atoms, err := nnf.Normalize(node.formula)
	if err != nil {
		return err
	}
	if node.atoms != nil {
		if err = node.atoms.Close(); err != nil {
			return errors.Join(err, atoms.Close())
		}
	}
	node.atoms = atoms
	return nil
}
