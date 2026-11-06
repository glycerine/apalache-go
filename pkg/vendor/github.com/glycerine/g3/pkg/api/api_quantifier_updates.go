package api

// Source: src/ast/ast.cpp quantifier update overloads, reached by the public
// immutable AST update boundary in src/api/api_ast.cpp:733-766.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) ownUpdatedQuantifier(value ast.ASTHandle, update func(*ast.ASTManager) (ast.ASTHandle, error)) (ast.ASTRef, error) {
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if _, err = manager.GetQuantifierBody(value); err != nil {
		return ast.ASTRef{}, err
	}
	result, err := update(manager)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return manager.NewRef(result)
}

// UpdateQuantifierWeight rebuilds value with weight and returns an owning
// reference. The caller releases it through Manager.DelRef.
func (c *SMTContext) UpdateQuantifierWeight(value ast.ASTHandle, weight int32) (ast.ASTRef, error) {
	return c.ownUpdatedQuantifier(value, func(manager *ast.ASTManager) (ast.ASTHandle, error) {
		return manager.UpdateQuantifierWeight(value, weight)
	})
}

// UpdateQuantifierKind rebuilds value with a new kind and body.
func (c *SMTContext) UpdateQuantifierKind(value ast.ASTHandle, forall bool, body ast.ASTHandle) (ast.ASTRef, error) {
	return c.ownUpdatedQuantifier(value, func(manager *ast.ASTManager) (ast.ASTHandle, error) {
		return manager.UpdateQuantifierKind(value, forall, body)
	})
}

// UpdateQuantifierPatterns rebuilds value with positive patterns and body.
// Supplying positive patterns clears any existing no-pattern annotations.
func (c *SMTContext) UpdateQuantifierPatterns(value ast.ASTHandle, patterns []ast.ASTHandle, body ast.ASTHandle) (ast.ASTRef, error) {
	return c.ownUpdatedQuantifier(value, func(manager *ast.ASTManager) (ast.ASTHandle, error) {
		return manager.UpdateQuantifierPatterns(value, patterns, body)
	})
}

// UpdateQuantifierAnnotations rebuilds value with complete positive and
// negative pattern lists and a new body.
func (c *SMTContext) UpdateQuantifierAnnotations(value ast.ASTHandle, patterns, noPatterns []ast.ASTHandle, body ast.ASTHandle) (ast.ASTRef, error) {
	return c.ownUpdatedQuantifier(value, func(manager *ast.ASTManager) (ast.ASTHandle, error) {
		return manager.UpdateQuantifierAnnotations(value, patterns, noPatterns, body)
	})
}
