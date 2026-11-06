package opt

// Source: src/opt/opt_context.cpp retained box bounds and assignments.

import "math/big"

func cloneObjectiveValue(source ObjectiveValue) ObjectiveValue {
	result := source
	result.Arithmetic.Finite = source.Arithmetic.Finite.Clone()
	result.ArithmeticModel.Finite = source.ArithmeticModel.Finite.Clone()
	result.SoftCost = *new(big.Rat).Set(&source.SoftCost)
	result.SoftSatisfied = *new(big.Rat).Set(&source.SoftSatisfied)
	result.SoftLower = *new(big.Rat).Set(&source.SoftLower)
	result.SoftUpper = *new(big.Rat).Set(&source.SoftUpper)
	result.SourceLower = *new(big.Rat).Set(&source.SourceLower)
	result.SourceUpper = *new(big.Rat).Set(&source.SourceUpper)
	result.SoftAssignment = append([]bool(nil), source.SoftAssignment...)
	return result
}

func cloneObjectiveValues(source []ObjectiveValue) []ObjectiveValue {
	result := make([]ObjectiveValue, len(source))
	for i := range source {
		result[i] = cloneObjectiveValue(source[i])
	}
	return result
}
