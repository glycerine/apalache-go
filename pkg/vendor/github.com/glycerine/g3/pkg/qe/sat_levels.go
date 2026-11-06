package qe

// Source: src/qe/qe_sat_tactic.cpp, init_Ms and reset.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type satLevel struct {
	formula ast.ASTRef
	solver  *smtcore.SMTSolver
}
type satLevels struct {
	manager *ast.ASTManager
	levels  []satLevel
}

func newSATLevels(m *ast.ASTManager, config Config, params *uparams.ParamsRef, blocks int, formula ast.ASTHandle, checkpoint func() error) (result *satLevels, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || blocks < 0 {
		return nil, ast.ErrASTHandle
	}
	result = &satLevels{manager: m}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	truth, err := m.MkTrue()
	if err != nil {
		return result, err
	}
	for i := 0; i <= blocks; i++ {
		h := truth
		if i == blocks {
			h = formula
		}
		ref, e := m.NewRef(h)
		if e != nil {
			return result, e
		}
		result.levels = append(result.levels, satLevel{formula: ref})
		entry := &result.levels[len(result.levels)-1]
		entry.solver, e = smtcore.NewSMTSolver(m, params, "", smtcore.SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: model.ModelEvaluatorConfig{Context: config.Context, Globals: config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace}})
		if e != nil {
			return result, e
		}
		if e = entry.solver.SetProduceModels(true); e != nil {
			return result, e
		}
		if i == blocks {
			if e = entry.solver.AssertFormula(formula); e != nil {
				return result, e
			}
		}
	}
	if checkpoint != nil {
		if err = checkpoint(); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (s *satLevels) Close() error {
	if s == nil || s.manager == nil {
		return nil
	}
	for len(s.levels) > 0 {
		i := len(s.levels) - 1
		entry := &s.levels[i]
		if entry.solver != nil {
			if err := entry.solver.Close(); err != nil {
				return err
			}
			entry.solver = nil
		}
		if entry.formula != (ast.ASTRef{}) {
			if err := s.manager.DelRef(&entry.formula); err != nil {
				return err
			}
		}
		s.levels = s.levels[:i]
	}
	s.manager = nil
	return nil
}
