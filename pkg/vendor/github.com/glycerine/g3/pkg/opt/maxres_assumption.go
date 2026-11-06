package opt

// Source: src/opt/maxres.cpp new_assumption/add_soft.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *maxResEngine) addAssumption(expression ast.ASTHandle, weight *big.Rat) error {
	if weight == nil || weight.Sign() <= 0 {
		return errMaxResState
	}
	if old := e.weights[expression]; old != nil {
		old.Add(old, weight)
		return nil
	}
	e.weights[expression] = new(big.Rat).Set(weight)
	e.active = append(e.active, expression)
	e.stats.Assumptions++
	return nil
}

func (e *maxResEngine) removeAssumption(expression ast.ASTHandle) (*big.Rat, error) {
	weight := e.weights[expression]
	if weight == nil {
		return nil, errMaxResState
	}
	delete(e.weights, expression)
	for i, candidate := range e.active {
		if candidate == expression {
			e.active[i] = e.active[len(e.active)-1]
			e.active = e.active[:len(e.active)-1]
			return weight, nil
		}
	}
	return nil, errMaxResState
}
