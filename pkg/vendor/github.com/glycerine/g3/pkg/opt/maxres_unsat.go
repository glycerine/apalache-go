package opt

// Source: src/opt/maxres.cpp get_cores, process_unsat and correction pivot.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *maxResEngine) processUnsatCore(result *ArithmeticResult) (lbool.LBool, error) {
	core, err := e.solver.backend.UnsatCore()
	if err != nil {
		return lbool.Undef, fmt.Errorf("maxres unsat core: %w", err)
	}
	status, err := e.processUnsatRound(core, result)
	if err != nil {
		return lbool.Undef, fmt.Errorf("maxres process core round: %w", err)
	}
	return status, nil
}
