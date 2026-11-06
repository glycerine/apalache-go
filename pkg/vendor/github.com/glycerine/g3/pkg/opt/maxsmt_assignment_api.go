package opt

// Source: src/opt/maxsmt.h get_assignment.

import "github.com/glycerine/g3/pkg/ast"

func (s *ArithmeticSolver) SoftAssignment(result *ArithmeticResult, objective int) ([]bool, error) {
	if s == nil || result == nil || objective < 0 || objective >= len(s.objectives) || objective >= len(result.Values) || !s.objectives[objective].soft || !result.Values[objective].Soft {
		return nil, ast.ErrASTHandle
	}
	assignment := result.Values[objective].SoftAssignment
	if len(assignment) != len(s.objectives[objective].softTerms) {
		return nil, ast.ErrASTDomain
	}
	return append([]bool(nil), assignment...), nil
}
