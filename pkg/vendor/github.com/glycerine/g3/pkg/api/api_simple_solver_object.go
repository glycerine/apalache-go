package api

// Source: src/api/api_solver.cpp:65-73,249-262; src/api/api_solver.h.

import (
	"github.com/glycerine/g3/pkg/solver"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type APISimpleSolver struct {
	owner           *smtContextState
	solver          solver.Solver
	explicitContext *uparams.ParamsContext
	explicitParams  uparams.ParamsRef
	explicitReady   bool
	closed          bool
}

func (c *SMTContext) NewSimpleSolver() (*APISimpleSolver, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	value, err := c.command.NewSimpleSolver()
	if err != nil {
		return nil, err
	}
	return &APISimpleSolver{owner: c.smtContextState, solver: value}, nil
}

func (s *APISimpleSolver) open() error {
	if s == nil || s.owner == nil || s.solver == nil || s.closed || s.owner.closed || s.owner.command == nil {
		return ErrSMTContextOwner
	}
	_, err := s.solver.Manager()
	return err
}

func (s *APISimpleSolver) Close() error {
	if s == nil || s.closed {
		return nil
	}
	if s.solver == nil {
		return ErrSMTContextOwner
	}
	if s.explicitReady {
		if err := s.explicitContext.Del(&s.explicitParams); err != nil {
			return err
		}
		s.explicitReady = false
		s.explicitContext = nil
	}
	if err := s.solver.Close(); err != nil {
		return err
	}
	s.owner, s.solver = nil, nil
	s.closed = true
	return nil
}
