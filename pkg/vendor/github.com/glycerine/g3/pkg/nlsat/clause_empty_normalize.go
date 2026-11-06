package nlsat

// Source: src/nlsat/nlsat_solver.cpp:997-1002,1021-1030.

import "github.com/glycerine/g3/pkg/sat"

func normalizeClauseLiterals(literals []sat.Literal) []sat.Literal {
	if len(literals) != 0 {
		return literals
	}
	return []sat.Literal{FalseLiteral}
}
