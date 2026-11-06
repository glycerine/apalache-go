package opt

// Source: src/opt/pb_sls.cpp soft_holds.

import "github.com/glycerine/g3/pkg/ast"

func (s *maxResSLSCandidateSearch) satisfiedSoft(objective *arithmeticObjective) ([]ast.ASTHandle, error) {
	result := make([]ast.ASTHandle, 0, len(objective.softTerms))
	for i := range objective.softTerms {
		holds, err := s.search.SoftHolds(uint32(i))
		if err != nil {
			return nil, err
		}
		if holds {
			term, err := s.engine.solver.manager.RefGet(objective.softTerms[i])
			if err != nil {
				return nil, err
			}
			result = append(result, term)
		}
	}
	return result, nil
}
