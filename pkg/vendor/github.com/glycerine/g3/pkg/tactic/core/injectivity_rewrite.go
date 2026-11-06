package core

// Source: src/tactic/core/injectivity_tactic.cpp rewriter_eq_cfg.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type injectivityRewriteConfiguration struct {
	rewriter.DefaultConfiguration
	owner *InjectivityTactic
}

func (c *injectivityRewriteConfiguration) RewritePatterns() bool { return true }
func (c *injectivityRewriteConfiguration) FlatAssoc(ast.ASTHandle) (bool, error) {
	return false, nil
}
func (c *injectivityRewriteConfiguration) ReduceApp(decl ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	t := c.owner
	m := t.manager
	if err := m.SetRef(proof, ast.ASTHandle{}); err != nil {
		return rewriter.RewriteFailed, err
	}
	if args.Count() != 2 {
		return rewriter.RewriteFailed, nil
	}
	family, err := m.GetFamilyID(decl)
	if err != nil || family != m.BasicFamilyID() {
		return rewriter.RewriteFailed, err
	}
	op, err := m.GetDeclKind(decl)
	if err != nil || op != ast.ASTOpEq {
		return rewriter.RewriteFailed, err
	}
	left, err := args.At(0)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	right, err := args.At(1)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	leftDecl, leftArgs, leftApp, err := t.application(left)
	if err != nil || !leftApp || len(leftArgs) != 1 {
		return rewriter.RewriteFailed, err
	}
	rightDecl, rightArgs, rightApp, err := t.application(right)
	if err != nil || !rightApp || len(rightArgs) != 1 || leftDecl != rightDecl {
		return rewriter.RewriteFailed, err
	}
	injective, err := t.relations.Contains(leftDecl)
	if err != nil || !injective {
		return rewriter.RewriteFailed, err
	}
	leftSort, err := m.GetSort(leftArgs[0])
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	rightSort, err := m.GetSort(rightArgs[0])
	if err != nil || leftSort != rightSort {
		return rewriter.RewriteFailed, err
	}
	result, err := m.MkEq(leftArgs[0], rightArgs[0])
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if err = m.SetRef(out, result); err != nil {
		return rewriter.RewriteFailed, err
	}
	return rewriter.RewriteDone, nil
}
