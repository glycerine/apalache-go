package opt

// Source: src/opt/maxres.cpp add_upper_bound_block parameter gate.
func (e *maxResEngine) upperBoundEligible() bool {
	return e != nil && e.objective != nil && e.config.addUpperBoundBlock && len(e.objective.softTerms) != 0
}
