package api

// Source: src/api/api_solver.cpp:169-180 and src/sat/dimacs.cpp.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/sat"
)

func (s *APISimpleSolver) LoadDIMACS(input io.Reader) (resultErr error) {
	if err := s.open(); err != nil {
		return err
	}
	if input == nil {
		return ErrSMTContextOwner
	}
	cnf, err := sat.ReadDIMACSCNF(input)
	if err != nil {
		return err
	}
	manager, err := s.solver.Manager()
	if err != nil {
		return err
	}
	assertions, err := s.buildDIMACSAssertions(manager, cnf)
	if err != nil {
		return err
	}
	defer func() {
		for i := range assertions {
			resultErr = errors.Join(resultErr, manager.DelRef(&assertions[i]))
		}
	}()
	for i := range assertions {
		formula, refErr := manager.RefGet(assertions[i])
		if refErr != nil {
			return refErr
		}
		if err = s.solver.Assert(formula); err != nil {
			return err
		}
	}
	return nil
}
