package api

// Source: src/api/api_solver.cpp:265-338.

import "github.com/glycerine/g3/pkg/ast"

func (s *APISimpleSolver) Push() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.solver.Push()
}

func (s *APISimpleSolver) Pop(count uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.solver.Pop(count)
}

func (s *APISimpleSolver) Assert(formula ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.solver.Assert(formula)
}

func (s *APISimpleSolver) AssertTracked(formula, tag ast.ASTHandle) error {
	if err := s.open(); err != nil {
		return err
	}
	return s.solver.AssertTracked(formula, tag)
}
