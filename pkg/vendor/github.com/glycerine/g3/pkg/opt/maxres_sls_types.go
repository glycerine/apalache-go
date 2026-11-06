package opt

// Source: src/opt/pb_sls.h and src/opt/maxsmt.h retained collaboration.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type maxResSLSCandidateSearch struct {
	engine *maxResEngine
	search *PBSLS
}

func joinMaxResSLSErrors(left, right error) error { return errors.Join(left, right) }

func (s *maxResSLSCandidateSearch) close() error {
	if s == nil || s.search == nil {
		return nil
	}
	err := s.search.Close()
	s.search = nil
	return err
}

func (s *maxResSLSCandidateSearch) hardAssertions() ([]ast.ASTHandle, error) {
	n, err := s.engine.solver.backend.NumAssertions()
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		result[i], err = s.engine.solver.backend.Assertion(i)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
