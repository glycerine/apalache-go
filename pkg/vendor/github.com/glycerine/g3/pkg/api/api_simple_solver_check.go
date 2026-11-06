package api

// Source: src/api/api_solver.cpp:376-391,443-449.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *APISimpleSolver) Check(assumptions ...ast.ASTHandle) (lbool.LBool, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, err
	}
	return s.solver.CheckSat(assumptions)
}

func (s *APISimpleSolver) ReasonUnknown() (string, error) {
	if err := s.open(); err != nil {
		return "", err
	}
	return s.solver.ReasonUnknown()
}
