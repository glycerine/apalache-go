package opt

// Source: src/opt/maxsmt.cpp commit_assignment weight accumulation.

import (
	"errors"
	"math/big"
)

var errMaxSMTAssignment = errors.New("opt: invalid MaxSMT assignment")

func softAssignmentWeight(objective *arithmeticObjective, assignment []bool) (*big.Rat, error) {
	if objective == nil || len(assignment) != len(objective.softWeights) {
		return nil, errMaxSMTAssignment
	}
	weight := new(big.Rat)
	for i, truth := range assignment {
		if truth {
			weight.Add(weight, objective.softWeights[i])
		}
	}
	return weight, nil
}
