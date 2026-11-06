package opt

// Source: src/opt/maxres.cpp is_literal.

import "github.com/glycerine/g3/pkg/ast"

func maxResLiteral(manager *ast.ASTManager, expression ast.ASTHandle) (bool, error) {
	count, err := manager.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	if count == 0 {
		return true, nil
	}
	matched, child, err := manager.IsNot(expression)
	if err != nil || !matched {
		return false, err
	}
	count, err = manager.GetNumArgs(child)
	return count == 0, err
}
