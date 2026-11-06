package base

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *DatalogContext) addQueryRule(body, predicate ast.ASTHandle, sorts []ast.ASTHandle, names []usymbol.Symbol) (resultErr error) {
	arguments := make([]ast.ASTHandle, len(sorts))
	var err error
	for index := range sorts {
		arguments[index], err = c.manager.MkVar(uint32(len(sorts)-index-1), sorts[index])
		if err != nil {
			return err
		}
	}
	head, err := c.manager.MkApp(predicate, arguments)
	if err != nil {
		return err
	}
	formula, err := c.manager.MkImplies(body, head)
	if err != nil {
		return err
	}
	if len(sorts) != 0 {
		formula, err = c.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: formula})
		if err != nil {
			return err
		}
	}
	return c.ruleManager.AddFormula(c.rules, formula, "")
}

func closeQueryRefs(manager *ast.ASTManager, refs ...ast.ASTRef) (result error) {
	for index := len(refs) - 1; index >= 0; index-- {
		if refs[index] != (ast.ASTRef{}) {
			result = errors.Join(result, manager.DelRef(&refs[index]))
		}
	}
	return result
}
