package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

// Simplify follows nlarith_plugin::simplify: factor the formula, then apply
// the theory rewriter, reporting whether the plugin changed the expression.
func (p *NonlinearArithmeticPlugin) Simplify(formula ast.ASTRef, rw *astrewriter.TheoryRewriter) (changed bool, err error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if formula == (ast.ASTRef{}) || rw == nil {
		return false, ast.ErrASTHandle
	}
	before, err := p.manager.RefGet(formula)
	if err != nil {
		return false, err
	}
	factored, err := p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, p.manager.DelRef(&factored)) }()
	if err = p.factor.Rewrite(before, factored); err != nil {
		return false, err
	}
	factoredFormula, err := p.manager.RefGet(factored)
	if err != nil {
		return false, err
	}
	if err = rw.Rewrite(factoredFormula, formula); err != nil {
		return false, err
	}
	after, err := p.manager.RefGet(formula)
	if err != nil {
		return false, err
	}
	return before != after, nil
}
