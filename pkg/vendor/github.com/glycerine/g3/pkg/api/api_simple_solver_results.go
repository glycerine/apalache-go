package api

// Source: src/api/api_solver.cpp:394-464.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func (s *APISimpleSolver) Model() (*APIModel, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	core, err := s.solver.AcquireModel()
	if err != nil {
		return nil, err
	}
	if core == nil {
		return nil, ast.ErrASTDomain
	}
	result, err := newAPIModel(s.owner, core, true)
	if err != nil {
		return nil, errors.Join(err, core.DecRef())
	}
	return result, nil
}

func (s *APISimpleSolver) Proof() (ast.ASTRef, error) {
	if err := s.open(); err != nil {
		return ast.ASTRef{}, err
	}
	value, err := s.solver.Proof()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if value.IsNull() {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	manager, err := s.solver.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return manager.NewRef(value)
}

func (s *APISimpleSolver) UnsatCore() (result []ast.ASTRef, resultErr error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	values, err := s.solver.UnsatCore()
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

func (s *APISimpleSolver) Statistics() (*APIStatistics, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	var value statistics.Statistics
	if err := s.solver.CollectStatistics(&value); err != nil {
		return nil, err
	}
	manager, err := s.solver.Manager()
	if err != nil {
		return nil, err
	}
	if numerals := manager.NumeralService(); numerals != nil && numerals.RuntimeOwner() != nil {
		if process := numerals.RuntimeOwner().ProcessOwner(); process != nil {
			if err = statistics.AddMemory(&value, process); err != nil {
				return nil, err
			}
		}
	}
	statistics.AddResourceLimit(&value, manager.ResourceLimit().Count())
	return newAPIStatistics(s.owner, value)
}
