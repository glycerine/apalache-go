package opt

// Source: src/opt/maxres.cpp mus_solver/primal_dual_solver l_true branches.

import "github.com/glycerine/g3/pkg/ast"

func (e *maxResEngine) processSearchSAT(result *ArithmeticResult, full bool) (bool, error) {
	if !full {
		return false, nil
	}
	if e.strategy == maxResPrimalDual && len(e.currentCorrection) != 0 {
		if err := e.processCorrection(append([]ast.ASTHandle(nil), e.currentCorrection...)); err != nil {
			return false, err
		}
		return false, nil
	}
	if e.strategy == maxResPrimalDual {
		// primal_dual_solver closes the remaining bound gap when the current
		// model has no correction set.
		e.lower.Set(&e.upper)
	}
	result.Values[e.index].SoftCost.Set(&e.lower)
	return true, nil
}
