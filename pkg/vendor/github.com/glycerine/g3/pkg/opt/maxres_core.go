package opt

// Source: src/opt/maxres.cpp get_cores/process_unsat.

import "github.com/glycerine/g3/pkg/ast"

func (e *maxResEngine) filterCore(core []ast.ASTHandle) ([]ast.ASTHandle, error) {
	seen := make(map[ast.ASTHandle]struct{}, len(core))
	filtered := make([]ast.ASTHandle, 0, len(core))
	for _, expression := range core {
		if e.weights[expression] == nil {
			continue
		}
		if _, duplicate := seen[expression]; duplicate {
			continue
		}
		seen[expression] = struct{}{}
		filtered = append(filtered, expression)
	}
	return filtered, nil
}
