package opt

import (
	"testing"

	"github.com/glycerine/g3/pkg/ast"
)

func evalMaxResFormula(t *testing.T, manager *ast.ASTManager, expression ast.ASTHandle, values map[ast.ASTHandle]bool) bool {
	t.Helper()
	if value, found := values[expression]; found {
		return value
	}
	if matched, child, err := manager.IsNot(expression); err != nil {
		t.Fatal(err)
	} else if matched {
		return !evalMaxResFormula(t, manager, child, values)
	}
	kind, err := manager.GetDeclKind(expression)
	if err != nil {
		t.Fatal(err)
	}
	count, err := manager.GetNumArgs(expression)
	if err != nil {
		t.Fatal(err)
	}
	result := kind == ast.ASTOpAnd
	for i := uint32(0); i < count; i++ {
		argument, err := manager.GetArg(expression, i)
		if err != nil {
			t.Fatal(err)
		}
		value := evalMaxResFormula(t, manager, argument, values)
		if kind == ast.ASTOpAnd {
			result = result && value
		} else if kind == ast.ASTOpOr {
			result = result || value
		} else {
			t.Fatal("unexpected MaxRes formula", kind)
		}
	}
	return result
}
