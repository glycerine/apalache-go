package api

// Source: src/api/api_solver.cpp Z3_solver_translate.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Translate copies solver state into target. Cross-context translation supports direct SMT, finite-domain, strategic, and
// supported caller-tactic solvers;
// same-context translation supports solvers with a tactic translation. Close the returned solver
// before closing target.
func (s *APISimpleSolver) Translate(target *SMTContext) (result *APISimpleSolver, resultErr error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if err := target.open(); err != nil {
		return nil, err
	}
	manager, err := target.Manager()
	if err != nil {
		return nil, err
	}
	config, err := target.command.SolverTranslationConfig()
	if err != nil {
		return nil, err
	}
	sourceGlobals, err := s.owner.command.GlobalParameters()
	if err != nil {
		return nil, err
	}
	var translatedParams uparams.ParamsRef
	if s.explicitReady {
		translatedParams, err = sourceGlobals.Context.TranslateTo(&s.explicitParams, config.Params)
	} else {
		translatedParams, err = config.Params.NewValue()
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, config.Params.Del(&translatedParams))
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	sourceManager, err := s.solver.Manager()
	if err != nil {
		return nil, err
	}
	var translated solver.Solver
	if sourceManager == manager {
		translated, err = s.solver.Translate(manager, &translatedParams)
	} else if direct, ok := s.solver.(*smtcore.SMTSolver); ok {
		translated, err = direct.TranslateWithConfig(manager, &translatedParams, config)
	} else if finite, ok := s.solver.(*portfolio.BoundedInt2BVSolver); ok {
		translated, err = finite.TranslateFiniteDomain(manager, &translatedParams, config.ModelEvaluator)
	} else if tactical, ok := s.solver.(*solver.TacticSolver); ok {
		translated, err = translateCallerTacticSolver(tactical, target, manager, &translatedParams, config)
	} else if strategic, ok := s.solver.(*solver.CombinedSolver); ok {
		translated, err = translateStrategicSolver(strategic, manager, &translatedParams, config)
	} else {
		return nil, fmt.Errorf("api: cross-context translation of this solver implementation is not yet supported")
	}
	if err != nil {
		return nil, err
	}
	return &APISimpleSolver{owner: target.smtContextState, solver: translated}, nil
}
