package opt

// Source: src/opt/maxsmt.cpp maxsmt_solver_base::find_mutexes and
// src/opt/maxres.cpp init_local.

import (
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *maxResEngine) preprocessMutexes(objective *arithmeticObjective) ([]maxResWeightedTerm, error) {
	expressions := make([]ast.ASTHandle, len(objective.softTerms))
	weights := make(map[ast.ASTHandle]*big.Rat, len(objective.softTerms))
	order := make([]ast.ASTHandle, 0, len(objective.softTerms))
	for i := range objective.softTerms {
		expression, err := e.solver.manager.RefGet(objective.softTerms[i])
		if err != nil {
			return nil, err
		}
		expressions[i] = expression
		if weights[expression] == nil {
			order = append(order, expression)
		}
		addMaxResMutexWeight(weights, expression, objective.softWeights[i])
	}
	status, mutexes, err := e.solver.backend.FindMutexes(expressions)
	if err != nil {
		return nil, fmt.Errorf("maxres mutex discovery: %w", err)
	}
	if status != lbool.True {
		return nil, errMaxResState
	}
	for _, mutex := range mutexes {
		if err = e.transformMutex(mutex, weights, &order); err != nil {
			return nil, fmt.Errorf("maxres mutex transform: %w", err)
		}
	}
	terms := make([]maxResWeightedTerm, 0, len(weights))
	seen := make(map[ast.ASTHandle]struct{}, len(order))
	for _, expression := range order {
		if _, duplicate := seen[expression]; duplicate {
			continue
		}
		seen[expression] = struct{}{}
		weight := weights[expression]
		if weight == nil || weight.Sign() <= 0 {
			continue
		}
		term := maxResWeightedTerm{expression: expression}
		term.weight.Set(weight)
		terms = append(terms, term)
	}
	return terms, nil
}
