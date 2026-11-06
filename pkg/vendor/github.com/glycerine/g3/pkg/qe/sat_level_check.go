package qe

// Source: src/qe/qe_sat_tactic.cpp, is_sat.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type satLevelResult struct {
	manager     *ast.ASTManager
	sat         bool
	assignments []ast.ASTRef
	model       *model.ModelCore
}

func (s *satLevels) check(index uint32, context ast.ASTHandle) (result *satLevelResult, err error) {
	if s == nil || s.manager == nil {
		return nil, ast.ErrASTHandle
	}
	if index >= uint32(len(s.levels)) {
		return nil, arena.ErrBounds
	}
	result = &satLevelResult{manager: s.manager}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	solver := s.levels[index].solver
	if err = solver.Push(); err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, solver.Pop(1)) }()
	if err = solver.AssertFormula(context); err != nil {
		return result, err
	}
	status, err := solver.CheckSat(nil)
	if err != nil {
		return result, err
	}
	assignments, err := solver.Assignments()
	if err != nil {
		return result, err
	}
	for _, h := range assignments {
		ref, e := s.manager.NewRef(h)
		if e != nil {
			return result, e
		}
		result.assignments = append(result.assignments, ref)
	}
	if status == lbool.Undef {
		reason, e := solver.ReasonUnknown()
		if e != nil {
			return result, e
		}
		return result, tactic.NewTacticException(reason)
	}
	result.sat = status == lbool.True
	if result.sat && index == 0 {
		result.model, err = solver.AcquireModel()
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

func (r *satLevelResult) Close() error {
	if r == nil || r.manager == nil {
		return nil
	}
	if r.model != nil {
		if err := r.model.DecRef(); err != nil {
			return err
		}
		r.model = nil
	}
	for len(r.assignments) > 0 {
		i := len(r.assignments) - 1
		if err := r.manager.DelRef(&r.assignments[i]); err != nil {
			return err
		}
		r.assignments = r.assignments[:i]
	}
	r.manager = nil
	return nil
}
