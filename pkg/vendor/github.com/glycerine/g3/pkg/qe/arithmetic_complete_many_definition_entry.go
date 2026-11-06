package qe

import "github.com/glycerine/g3/pkg/ast"

func (r *ArithmeticCompleteProjectionResult) definitionEntry(index uint32) (ast.ASTHandle, ast.ASTHandle, error) {
	variable, e := r.Variable(index)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, e
	}
	declaration, e := r.manager.GetDecl(variable)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, e
	}
	definition, e := r.Definition(index)
	return declaration, definition, e
}
