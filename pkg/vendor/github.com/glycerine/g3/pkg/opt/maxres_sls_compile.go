package opt

// Source: src/opt/pb_sls.cpp add hard/soft and opt_sls_solver.h pbsls_opt.

import "fmt"

func (s *maxResSLSCandidateSearch) compile(objective *arithmeticObjective) error {
	hard, err := s.hardAssertions()
	if err != nil {
		return err
	}
	for i, formula := range hard {
		if err = s.search.AddHard(formula); err != nil {
			return fmt.Errorf("maxres SLS hard assertion %d: %w", i, err)
		}
	}
	for i := range objective.softTerms {
		term, refErr := s.engine.solver.manager.RefGet(objective.softTerms[i])
		if refErr != nil {
			return refErr
		}
		if err = s.search.AddSoft(term, objective.softWeights[i]); err != nil {
			return fmt.Errorf("maxres SLS soft assertion %d: %w", i, err)
		}
	}
	return nil
}
