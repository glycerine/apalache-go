package opt

// Source: src/opt/maxsmt.cpp init/update_assignment/get_assignment.

import "math/big"

func setSoftObjectiveValue(result *ArithmeticResult, index int, cost, satisfied big.Rat, assignment []bool) {
	value := &result.Values[index]
	value.SoftCost.Set(&cost)
	value.SoftSatisfied.Set(&satisfied)
	value.SoftAssignment = append([]bool(nil), assignment...)
	value.Soft = true
}
