package opt

// Source: src/opt/maxres.cpp split_core.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *maxResEngine) splitCore(core []ast.ASTHandle) (*big.Rat, error) {
	if len(core) == 0 {
		return nil, errMaxResState
	}
	first := e.weights[core[0]]
	if first == nil || first.Sign() <= 0 {
		return nil, errMaxResState
	}
	minimum := new(big.Rat).Set(first)
	for _, expression := range core[1:] {
		weight := e.weights[expression]
		if weight == nil || weight.Sign() <= 0 {
			return nil, errMaxResState
		}
		if weight.Cmp(minimum) < 0 {
			minimum.Set(weight)
		}
	}
	for _, expression := range core {
		weight, err := e.removeAssumption(expression)
		if err != nil {
			return nil, err
		}
		if weight.Cmp(minimum) > 0 {
			residual := new(big.Rat).Sub(weight, minimum)
			if err = e.addAssumption(expression, residual); err != nil {
				return nil, err
			}
		}
	}
	return minimum, nil
}
