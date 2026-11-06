package opt

// Source: src/opt/maxres.cpp check_sat_hill_climb.

import "github.com/glycerine/g3/pkg/util/lbool"

func (e *maxResEngine) checkHillClimb(result *ArithmeticResult) (status lbool.LBool, full bool, err error) {
	boundaries := e.hillClimbBoundaries()
	for _, size := range boundaries {
		e.stats.Checks++
		status, err = e.solver.backend.CheckSat(e.active[:size])
		if err != nil || status != lbool.True {
			return status, false, err
		}
		if err = e.updateCandidate(result); err != nil {
			return lbool.Undef, false, err
		}
		if size == len(e.active) {
			return lbool.True, true, nil
		}
	}
	return lbool.True, len(e.active) == 0, nil
}
