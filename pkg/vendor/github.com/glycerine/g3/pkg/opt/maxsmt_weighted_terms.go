package opt

// Source: src/opt/sortmax.cpp unsigned-weight input expansion.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

const maxSortMaxInputs = 4096

type maxSMTWeightedTerm struct {
	expression ast.ASTHandle
	weight     big.Int
}

func maxSMTIntegralTerms(manager *ast.ASTManager, objective *arithmeticObjective) ([]maxSMTWeightedTerm, int, bool, error) {
	terms := make([]maxSMTWeightedTerm, len(objective.softTerms))
	total := 0
	for i := range objective.softTerms {
		if objective.softWeights[i].Denom().Cmp(big.NewInt(1)) != 0 || objective.softWeights[i].Sign() <= 0 || !objective.softWeights[i].Num().IsInt64() {
			return nil, 0, false, nil
		}
		weight := objective.softWeights[i].Num().Int64()
		if weight > maxSortMaxInputs || total > maxSortMaxInputs-int(weight) {
			return nil, 0, false, nil
		}
		expression, err := manager.RefGet(objective.softTerms[i])
		if err != nil {
			return nil, 0, false, err
		}
		terms[i].expression = expression
		terms[i].weight.SetInt64(weight)
		total += int(weight)
	}
	return terms, total, true, nil
}

func maxSMTExpandTerms(terms []maxSMTWeightedTerm, capacity int) []ast.ASTHandle {
	inputs := make([]ast.ASTHandle, 0, capacity)
	for i := range terms {
		for count := terms[i].weight.Int64(); count > 0; count-- {
			inputs = append(inputs, terms[i].expression)
		}
	}
	return inputs
}
