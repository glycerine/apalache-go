package core

// Source: src/tactic/core/blast_term_ite_tactic.cpp rw_cfg::mk_app_core.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (c *blastTermITEConfiguration) ReduceApp(decl ast.ASTHandle, arguments ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	m := c.owner.manager
	if err := m.SetRef(proof, ast.ASTHandle{}); err != nil {
		return rewriter.RewriteFailed, err
	}
	family, err := m.GetFamilyID(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	op, err := m.GetDeclKind(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if family == m.BasicFamilyID() && op == ast.ASTOpIte {
		return rewriter.RewriteFailed, nil
	}
	args, err := blastTermITEArguments(m, arguments)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	for i, arg := range args {
		boolean, err := blastTermITEIsBool(m, arg)
		if err != nil {
			return rewriter.RewriteFailed, err
		}
		if boolean {
			continue
		}
		condition, thenPart, elsePart, ite, err := blastTermITEParts(m, arg)
		if err != nil {
			return rewriter.RewriteFailed, err
		}
		if !ite {
			continue
		}
		if err = c.traceApplication(decl, args, out); err != nil {
			return rewriter.RewriteFailed, err
		}
		thenArgs := append([]ast.ASTHandle(nil), args...)
		thenArgs[i] = thenPart
		thenApp, err := m.MkApp(decl, thenArgs)
		if err != nil {
			return rewriter.RewriteFailed, err
		}
		c.owner.numFresh++
		equal, err := m.AreEqual(thenPart, elsePart)
		if err != nil {
			return rewriter.RewriteFailed, err
		}
		if equal {
			if err = m.SetRef(out, thenApp); err != nil {
				return rewriter.RewriteFailed, err
			}
			return rewriter.Rewrite1, nil
		}
		elseArgs := append([]ast.ASTHandle(nil), args...)
		elseArgs[i] = elsePart
		elseApp, err := m.MkApp(decl, elseArgs)
		if err != nil {
			return rewriter.RewriteFailed, err
		}
		result, err := m.MkIte(condition, thenApp, elseApp)
		if err != nil {
			return rewriter.RewriteFailed, err
		}
		if err = m.SetRef(out, result); err != nil {
			return rewriter.RewriteFailed, err
		}
		return rewriter.Rewrite3, nil
	}
	return rewriter.RewriteFailed, nil
}
