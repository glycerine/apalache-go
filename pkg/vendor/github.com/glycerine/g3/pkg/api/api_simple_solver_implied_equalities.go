package api

// Source: src/api/api_solver.cpp:479-492.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *APISimpleSolver) ImpliedEqualities(terms []ast.ASTHandle) (lbool.LBool, []uint32, error) {
	if err := s.open(); err != nil {
		return lbool.Undef, nil, err
	}
	return smtcore.ImpliedEqualities(s.solver, terms)
}
