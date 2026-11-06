package opt

// Source: src/opt/maxres.cpp maxres::operator(), mus_solver and found_optimum.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/lbool"
)

const maxResIterationLimit = 10000

func (e *maxResEngine) optimize(result *ArithmeticResult) (status lbool.LBool, optimal bool, err error) {
	if e == nil || e.solver == nil || e.solver.backend == nil || result == nil {
		return lbool.Undef, false, errMaxResState
	}
	defer func() {
		if e.bestSet && e.index >= 0 && e.index < len(result.Values) {
			value := &result.Values[e.index]
			value.SoftLower.Set(&e.lower)
			value.SoftUpper.Set(&e.upper)
			value.SoftBoundsValid = true
		}
	}()
	if err = e.trySLSCandidate(result); err != nil {
		return lbool.Undef, false, err
	}
	// maxsmt_solver_base::init reads its model captured before MaxRes mutex
	// discovery. Those preprocessing checks must not replace that candidate.
	if result.Model == nil {
		return lbool.Undef, false, fmt.Errorf("maxres source initial model missing: %w", errMaxResState)
	}
	if err = result.Model.IncRef(); err != nil {
		return lbool.Undef, false, err
	}
	if err = e.updateCandidateModel(result, result.Model); err != nil {
		return lbool.Undef, false, fmt.Errorf("maxres initial candidate: %w", err)
	}
	e.sortAssumptions()
	for {
		if e.optimalByBounds() {
			if err = e.publishOptimum(result); err != nil {
				return lbool.Undef, false, err
			}
			return lbool.False, true, nil
		}
		if len(e.active) == 0 {
			if err = e.publishOptimum(result); err != nil {
				return lbool.Undef, false, err
			}
			return lbool.False, true, nil
		}
		var full bool
		status, full, err = e.checkHillClimb(result)
		if err != nil {
			return lbool.Undef, false, err
		}
		switch status {
		case lbool.True:
			done, e2 := e.processSearchSAT(result, full)
			if e2 != nil {
				return lbool.Undef, false, fmt.Errorf("maxres satisfiable round: %w", e2)
			}
			if done {
				return lbool.False, true, nil
			}
		case lbool.False:
			roundStatus, e2 := e.processSearchUNSAT(result)
			if e2 != nil {
				return lbool.Undef, false, e2
			}
			if roundStatus == lbool.False {
				return lbool.False, true, nil
			}
			if roundStatus == lbool.Undef {
				return lbool.Undef, false, nil
			}
		case lbool.Undef:
			return status, false, nil
		default:
			return lbool.Undef, false, errMaxResState
		}
	}
}
