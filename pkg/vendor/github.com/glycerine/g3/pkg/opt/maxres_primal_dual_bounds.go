package opt

// Source: src/opt/maxres.cpp process_unsat primal-dual lower-bound clamp.

func (e *maxResEngine) clampPrimalDualLower() {
	if e.strategy == maxResPrimalDual && e.bestSet && e.lower.Cmp(&e.upper) > 0 {
		e.lower.Set(&e.upper)
	}
}
