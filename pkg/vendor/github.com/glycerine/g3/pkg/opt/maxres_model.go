package opt

// Source: src/opt/maxres.cpp update_assignment/found_optimum.

func (e *maxResEngine) updateResult(result *ArithmeticResult) error {
	return e.updateCandidate(result)
}
