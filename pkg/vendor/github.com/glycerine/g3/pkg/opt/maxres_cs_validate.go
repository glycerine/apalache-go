package opt

// Source: src/opt/maxres.cpp correction-set active-membership guards.

import "github.com/glycerine/g3/pkg/ast"

func (e *maxResEngine) validCorrection(correction []ast.ASTHandle) bool {
	if len(correction) == 0 {
		return false
	}
	seen := make(map[ast.ASTHandle]struct{}, len(correction))
	for _, expression := range correction {
		weight := e.weights[expression]
		if weight == nil || weight.Sign() <= 0 {
			return false
		}
		if _, duplicate := seen[expression]; duplicate {
			return false
		}
		seen[expression] = struct{}{}
	}
	return true
}
