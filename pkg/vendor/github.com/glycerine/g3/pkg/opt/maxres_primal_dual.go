package opt

// Source: src/opt/maxres.cpp primal_dual_solver. It retains the source's
// alternating core/correction-set progress and clamps weighted lower estimates
// to the best published upper bound.

import "github.com/glycerine/g3/pkg/util/lbool"

func (e *maxResEngine) optimizePrimalDual(result *ArithmeticResult) (lbool.LBool, bool, error) {
	if err := e.trySLSCandidate(result); err != nil {
		return lbool.Undef, false, err
	}
	for iteration := 0; iteration < maxResIterationLimit; iteration++ {
		status, full, err := e.checkHillClimb(result)
		if err != nil || status == lbool.Undef {
			return status, false, err
		}
		if status == lbool.True && full {
			if err = e.updateResult(result); err != nil {
				return lbool.Undef, false, err
			}
			e.lower.Set(&e.upper)
			return lbool.False, true, nil
		}
		rawCore, err := e.solver.backend.UnsatCore()
		if err != nil {
			return lbool.Undef, false, err
		}
		var roundStatus lbool.LBool
		if roundStatus, err = e.processUnsatRound(rawCore, result); err != nil {
			return lbool.Undef, false, err
		}
		if roundStatus == lbool.Undef {
			return lbool.Undef, false, nil
		}
		if roundStatus == lbool.False {
			e.lower.Set(&e.upper)
			return lbool.False, true, nil
		}
		e.clampPrimalDualLower()
	}
	return lbool.Undef, false, errMaxResState
}
