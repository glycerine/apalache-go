package opt

// Source: src/opt/maxres.cpp found_optimum and lower/upper loop guard.

func (e *maxResEngine) optimalByBounds() bool {
	return e.bestSet && e.lower.Cmp(&e.upper) >= 0
}

func (e *maxResEngine) publishOptimum(result *ArithmeticResult) error {
	if e == nil || result == nil || e.index < 0 || e.index >= len(result.Values) || !e.bestSet {
		return errMaxResState
	}
	result.Values[e.index].SoftCost.Set(&e.lower)
	return nil
}
