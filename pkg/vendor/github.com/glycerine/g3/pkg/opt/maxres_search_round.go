package opt

// Source: src/opt/maxres.cpp process_unsat/get_cores round orchestration.

import "github.com/glycerine/g3/pkg/ast"
import "github.com/glycerine/g3/pkg/util/lbool"

func (e *maxResEngine) processUnsatRound(raw []ast.ASTHandle, result *ArithmeticResult) (lbool.LBool, error) {
	cores, status, err := e.collectCoreBatch(raw, result)
	if err != nil {
		return lbool.Undef, err
	}
	if status != lbool.True {
		return status, nil
	}
	if err = e.processCoreBatch(cores); err != nil {
		return lbool.Undef, err
	}
	return lbool.True, nil
}
