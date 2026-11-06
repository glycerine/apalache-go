package opt

// Source: src/opt/maxres.cpp process_sat minimum correction weight.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *maxResEngine) correctionWeight(correction []ast.ASTHandle) (*big.Rat, error) {
	if len(correction) == 0 {
		return nil, errMaxResState
	}
	minimum := e.weights[correction[0]]
	if minimum == nil || minimum.Sign() <= 0 {
		return nil, errMaxResState
	}
	answer := new(big.Rat).Set(minimum)
	for _, expression := range correction[1:] {
		weight := e.weights[expression]
		if weight == nil || weight.Sign() <= 0 {
			return nil, errMaxResState
		}
		if weight.Cmp(answer) < 0 {
			answer.Set(weight)
		}
	}
	return answer, nil
}
