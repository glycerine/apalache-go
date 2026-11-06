package api

// Source: src/api/api_solver.cpp:296-338.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
)

func (s *APISimpleSolver) ScopeLevel() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	return s.solver.ScopeLevel()
}

// Assertions returns independently owning references, matching the native AST
// vector returned by Z3_solver_get_assertions.
func (s *APISimpleSolver) Assertions() (result []ast.ASTRef, resultErr error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	values, err := solver.Assertions(s.solver)
	if err != nil {
		return nil, err
	}
	manager, err := s.solver.Manager()
	if err != nil {
		return nil, err
	}
	for _, value := range values {
		ref, refErr := manager.NewRef(value)
		if refErr != nil {
			for i := range result {
				resultErr = errors.Join(resultErr, manager.DelRef(&result[i]))
			}
			return nil, errors.Join(refErr, resultErr)
		}
		result = append(result, ref)
	}
	return result, nil
}
