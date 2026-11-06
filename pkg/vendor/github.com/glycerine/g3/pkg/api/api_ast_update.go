package api

// Source: src/api/api_ast.cpp:733-766.

import "github.com/glycerine/g3/pkg/ast"

// UpdateTerm rebuilds an application with new arguments, or a quantifier with
// a new body. Other expression kinds are returned unchanged. The returned
// reference is owning and must be released with Manager.DelRef.
//
// Returning an ASTRef is the Go ownership bridge for the native context's
// save_ast_trail call: the result remains live independently of temporary
// builders, without adding a hidden shared trail to SMTContext.
func (c *SMTContext) UpdateTerm(term ast.ASTHandle, arguments ...ast.ASTHandle) (ast.ASTRef, error) {
	m, err := c.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	kind, err := m.GetKind(term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	result := term
	switch kind {
	case ast.ASTApp:
		count, queryErr := m.GetNumArgs(term)
		if queryErr != nil {
			return ast.ASTRef{}, queryErr
		}
		if uint64(len(arguments)) != uint64(count) {
			return ast.ASTRef{}, ast.ErrASTDomain
		}
		declaration, queryErr := m.GetDecl(term)
		if queryErr != nil {
			return ast.ASTRef{}, queryErr
		}
		result, err = m.MkApp(declaration, arguments)
	case ast.ASTQuantifier:
		if len(arguments) != 1 {
			return ast.ASTRef{}, ast.ErrASTDomain
		}
		result, err = m.UpdateQuantifierBody(term, arguments[0])
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	return m.NewRef(result)
}
