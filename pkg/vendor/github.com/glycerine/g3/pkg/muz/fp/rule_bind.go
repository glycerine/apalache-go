package fp

// Source: src/muz/fp/dl_cmds.cpp dl_context::add_rule collection.

import "github.com/glycerine/g3/pkg/ast"

// BindRule replaces declared fixedpoint variables and returns an owned formula.
func (e *PropositionalEngine) BindRule(formula ast.ASTHandle) (ast.ASTRef, error) {
	if err := e.open(); err != nil {
		return ast.ASTRef{}, err
	}
	return e.binder.Bind(formula, true)
}
