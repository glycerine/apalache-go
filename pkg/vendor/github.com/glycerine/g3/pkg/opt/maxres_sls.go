package opt

// Source: src/opt/opt_context.cpp enable_sls and src/opt/pb_sls.cpp.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *maxResEngine) trySLSCandidate(result *ArithmeticResult) (resultErr error) {
	if !e.config.enableSLS || len(e.objective.softTerms) == 0 {
		return nil
	}
	e.stats.SLSAttempts++
	search, err := e.newSLSCandidateSearch(result)
	if err != nil {
		return fmt.Errorf("maxres SLS construct: %w", err)
	}
	defer func() { resultErr = joinMaxResSLSErrors(resultErr, search.close()) }()
	if err = search.compile(e.objective); err != nil {
		return fmt.Errorf("maxres SLS compile: %w", err)
	}
	status, err := search.check()
	if err != nil {
		return fmt.Errorf("maxres SLS search: %w", err)
	}
	if status != lbool.True {
		return nil
	}
	assumptions, err := search.satisfiedSoft(e.objective)
	if err != nil {
		return fmt.Errorf("maxres SLS assignment: %w", err)
	}
	if len(assumptions) == 0 {
		return nil
	}
	e.stats.SLSChecks++
	status, err = e.solver.backend.CheckSat(assumptions)
	if err != nil {
		return fmt.Errorf("maxres SLS validate: %w", err)
	}
	if status != lbool.True {
		return nil
	}
	if err = e.updateCandidate(result); err != nil {
		return fmt.Errorf("maxres SLS candidate: %w", err)
	}
	return nil
}
