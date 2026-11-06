package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func CollectArithmeticAtomsNNF(manager *ast.ASTManager, formula ast.ASTHandle) (result *ArithmeticAtomSet, resultErr error) {
	result, e := NewArithmeticAtomSet(manager)
	if e != nil {
		return nil, e
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	c := arithmeticAtomCollector{manager: manager, result: result, seen: make(map[arithmeticAtomVisit]bool)}
	if e = c.walk(formula, true); e != nil {
		return nil, e
	}
	return result, nil
}
