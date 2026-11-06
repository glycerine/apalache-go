package opt

// Source: src/opt/maxsmt.cpp get_lower/get_upper at optimal termination.

func finalizeSoftBounds(result *ArithmeticResult, index int) {
	value := &result.Values[index]
	value.SoftLower.Set(&value.SoftCost)
	value.SoftUpper.Set(&value.SoftCost)
	value.SoftBoundsValid = true
}
