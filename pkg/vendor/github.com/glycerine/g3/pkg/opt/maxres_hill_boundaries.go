package opt

// Source: src/opt/maxres.cpp check_sat_hill_climb assumption scheduling.

func (e *maxResEngine) hillClimbBoundaries() []int {
	e.sortAssumptions()
	boundaries := e.weightBoundaries()
	if !e.config.hillClimb && len(boundaries) != 0 {
		return boundaries[len(boundaries)-1:]
	}
	return boundaries
}
