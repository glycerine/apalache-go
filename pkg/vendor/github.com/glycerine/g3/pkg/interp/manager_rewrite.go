package interp

// Source checkpoint: src/interp/iz3mgr.cpp z3_simplify and z3_really_simplify.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

func (m *Manager) rewrite(term ast.ASTHandle, rewriter *astrewriter.TheoryRewriter) (ast.ASTRef, error) {
	if err := m.open(); err != nil {
		return ast.ASTRef{}, err
	}
	if rewriter == nil {
		return ast.ASTRef{}, NewError("interpolation manager has no theory rewriter")
	}
	out, err := m.ast.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = rewriter.Rewrite(term, out); err != nil {
		return ast.ASTRef{}, errors.Join(err, m.ast.DelRef(&out))
	}
	return out, nil
}

func (m *Manager) Simplify(term ast.ASTHandle) (ast.ASTRef, error) {
	return m.rewrite(term, m.simplifier)
}

func (m *Manager) ReallySimplify(term ast.ASTHandle) (ast.ASTRef, error) {
	return m.rewrite(term, m.strong)
}
