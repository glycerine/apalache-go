package opt

// Source: src/opt/opt_context.cpp get_lower_as_num/get_upper_as_num for O_MAXSMT.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (s *ArithmeticSolver) SoftBounds(result *ArithmeticResult, objective int) (lower, upper *big.Rat, err error) {
	if s == nil || result == nil || objective < 0 || objective >= len(s.objectives) || objective >= len(result.Values) || !s.objectives[objective].soft || !result.Values[objective].Soft {
		return nil, nil, ast.ErrASTHandle
	}
	value := &result.Values[objective]
	return new(big.Rat).Set(&value.SoftLower), new(big.Rat).Set(&value.SoftUpper), nil
}
