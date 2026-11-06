package opt

// Source: src/opt/maxsmt.h get_lower/get_upper/get_assignment result surface.

import "github.com/glycerine/g3/pkg/ast"

func (s *ArithmeticSolver) SoftValue(result *ArithmeticResult, objective int) (ObjectiveValue, error) {
	if s == nil || result == nil || objective < 0 || objective >= len(s.objectives) || objective >= len(result.Values) || !s.objectives[objective].soft || !result.Values[objective].Soft {
		return ObjectiveValue{}, ast.ErrASTHandle
	}
	return cloneObjectiveValue(result.Values[objective]), nil
}
