package api

// Source: src/api/api_solver.cpp:495-554.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *APISimpleSolver) Consequences(assumptions, variables []ast.ASTHandle) (status lbool.LBool, result []ast.ASTRef, resultErr error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	status, values, err := s.solver.GetConsequences(assumptions, variables)
	if err != nil {
		return status, nil, err
	}
	manager, err := s.solver.Manager()
	if err != nil {
		return status, nil, err
	}
	for _, value := range values {
		ref, refErr := manager.NewRef(value)
		if refErr != nil {
			for i := range result {
				resultErr = errors.Join(resultErr, manager.DelRef(&result[i]))
			}
			return lbool.Undef, nil, errors.Join(refErr, resultErr)
		}
		result = append(result, ref)
	}
	return status, result, nil
}
