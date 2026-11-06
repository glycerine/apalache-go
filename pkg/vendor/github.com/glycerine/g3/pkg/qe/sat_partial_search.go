package qe

// Source: src/qe/qe_sat_tactic.cpp, qt/extrapolate using partial projection.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type satPartialSearch struct {
	manager *ast.ASTManager
	config  Config
	options satTacticOptions
	alt     *satAlternation
	levels  *satLevels
	rw      *rewriter.TheoryRewriter
	nnf     *QENNFNormalizer
	aux     *smtcore.SMTSolver
	strong  *smtcore.ExprStrongContextSimplifier
}

// collectStatistics separates the per-application level solvers from the
// auxiliary solver and context simplifier, which correspond to the tactic's
// persistent source members.
func (s *satPartialSearch) collectStatistics(levels, persistent *statistics.Statistics) error {
	if s == nil || s.manager == nil || s.levels == nil || s.aux == nil {
		return arena.ErrClosed
	}
	if levels == nil || persistent == nil {
		return ast.ErrASTHandle
	}
	for _, level := range s.levels.levels {
		if err := level.solver.CollectStatistics(levels); err != nil {
			return err
		}
	}
	if err := s.aux.CollectStatistics(persistent); err != nil {
		return err
	}
	if s.strong != nil {
		if err := s.strong.CollectStatistics(persistent); err != nil {
			return err
		}
	}
	return nil
}

type satPartialResult struct {
	manager *ast.ASTManager
	formula ast.ASTRef
	model   *model.ModelCore
}

func newSATPartialSearch(m *ast.ASTManager, config Config, alt *satAlternation, options satTacticOptions) (result *satPartialSearch, err error) {
	if m == nil || alt == nil || alt.closed || alt.manager != m || config.Context == nil || config.Globals == nil || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	result = &satPartialSearch{manager: m, config: config, alt: alt, options: options}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	formula, err := m.RefGet(alt.formula)
	if err != nil {
		return result, err
	}
	result.levels, err = newSATLevels(m, config, nil, len(alt.variables), formula, result.checkpoint)
	if err != nil {
		return result, err
	}
	result.rw, err = rewriter.NewTheoryRewriter(m, config.Context, config.Globals, nil)
	if err != nil {
		return result, err
	}
	defaults := &satNNFDefault{manager: m}
	result.nnf, err = NewQENNFNormalizer(m, defaults, defaults, config)
	if err != nil {
		return result, err
	}
	result.aux, err = smtcore.NewSMTSolver(m, nil, "", smtcore.SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: model.ModelEvaluatorConfig{Context: config.Context, Globals: config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace}})
	if err != nil {
		return result, err
	}
	if options.strongSimplify {
		result.strong, err = smtcore.NewExprStrongContextSimplifier(m, smtcore.SMTSolverConfig{Params: config.Context, Globals: config.Globals, ModelEvaluator: model.ModelEvaluatorConfig{Context: config.Context, Globals: config.Globals, Memory: m.NumeralService().RuntimeOwner(), Cooperation: config.Cooperation, Trace: config.Trace}})
		if err != nil {
			return result, err
		}
	}
	return result, nil
}
func (s *satPartialSearch) checkpoint() error {
	canceled, err := s.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(s.manager.ResourceLimit().CancelMessage())
	}
	return s.config.Cooperation.Cooperate("qe-sat")
}
func (s *satPartialSearch) Close() error {
	if s == nil || s.manager == nil {
		return nil
	}
	if s.strong != nil {
		if err := s.strong.Close(); err != nil {
			return err
		}
		s.strong = nil
	}
	if s.aux != nil {
		if err := s.aux.Close(); err != nil {
			return err
		}
		s.aux = nil
	}
	if s.nnf != nil {
		if err := s.nnf.Close(); err != nil {
			return err
		}
		s.nnf = nil
	}
	if s.rw != nil {
		if err := s.rw.Close(); err != nil {
			return err
		}
		s.rw = nil
	}
	if s.levels != nil {
		if err := s.levels.Close(); err != nil {
			return err
		}
		s.levels = nil
	}
	s.manager = nil
	return nil
}
func (r *satPartialResult) Close() error {
	if r == nil || r.manager == nil {
		return nil
	}
	if r.model != nil {
		if err := r.model.DecRef(); err != nil {
			return err
		}
		r.model = nil
	}
	if r.formula != (ast.ASTRef{}) {
		if err := r.manager.DelRef(&r.formula); err != nil {
			return err
		}
	}
	r.manager = nil
	return nil
}

func (s *satPartialSearch) extrapolate(index uint32, context ast.ASTHandle, check *satLevelResult) (ast.ASTRef, error) {
	m := s.manager
	b, err := m.RefGet(s.levels.levels[index].formula)
	if err != nil {
		return ast.ASTRef{}, err
	}
	switch s.options.extrapolate {
	case 1:
		return satStrengthen(m, s.nnf, s.levels.levels[index].solver, s.rw, context, b, s.checkpoint)
	case 2:
		assignments := make([]ast.ASTHandle, len(check.assignments))
		for i, ref := range check.assignments {
			assignments[i], err = m.RefGet(ref)
			if err != nil {
				return ast.ASTRef{}, err
			}
		}
		return satCoreExtrapolate(m, s.aux, b, assignments, s.checkpoint)
	case 3:
		return satWeaken(m, s.nnf, s.aux, s.rw, context, b, s.checkpoint)
	default:
		return m.NewRef(b)
	}
}

func (s *satPartialSearch) qt(index uint32, context ast.ASTHandle) (result *satPartialResult, err error) {
	if s == nil || s.manager == nil || index >= uint32(len(s.levels.levels)) {
		return nil, ast.ErrASTHandle
	}
	m := s.manager
	for {
		if err = s.checkpoint(); err != nil {
			return nil, err
		}
		check, e := s.levels.check(index, context)
		if e != nil {
			return nil, e
		}
		if !check.sat {
			f, e := m.MkFalse()
			if e == nil {
				result = &satPartialResult{manager: m}
				result.formula, e = m.NewRef(f)
			}
			e = errors.Join(e, check.Close())
			if e != nil {
				if result != nil {
					e = errors.Join(e, result.Close())
				}
				return nil, e
			}
			return result, nil
		}
		ctxM, e := s.extrapolate(index, context, check)
		if e != nil {
			return nil, errors.Join(e, check.Close())
		}
		if index == uint32(len(s.alt.variables)) {
			result = &satPartialResult{manager: m, formula: ctxM, model: check.model}
			check.model = nil
			if e = check.Close(); e != nil {
				return nil, errors.Join(e, result.Close())
			}
			return result, nil
		}
		ctxValue, e := m.RefGet(ctxM)
		if e != nil {
			return nil, errors.Join(e, m.DelRef(&ctxM), check.Close())
		}
		child, e := s.qt(index+1, ctxValue)
		if e != nil {
			return nil, errors.Join(e, m.DelRef(&ctxM), check.Close())
		}
		childValue, e := m.RefGet(child.formula)
		if e != nil {
			return nil, errors.Join(e, child.Close(), m.DelRef(&ctxM), check.Close())
		}
		if m.IsFalse(childValue) {
			result = &satPartialResult{manager: m, formula: ctxM, model: check.model}
			check.model = nil
			e = errors.Join(child.Close(), check.Close())
			if e != nil {
				return nil, errors.Join(e, result.Close())
			}
			return result, nil
		}
		vars := make([]ast.ASTHandle, len(s.alt.variables[index]))
		for i, ref := range s.alt.variables[index] {
			vars[i], e = m.RefGet(ref)
			if e != nil {
				break
			}
		}
		if e == nil {
			e = projectSATPartialLevel(s.levels, index, vars, childValue, s.config, s.rw, s.strong, s.options.simplifyLocal, s.options.projection)
		}
		e = errors.Join(e, child.Close(), m.DelRef(&ctxM), check.Close())
		if e != nil {
			return nil, e
		}
	}
}
