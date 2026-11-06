package api

// Source: src/solver/combined_solver.cpp and src/solver/tactic2solver.cpp
// translation, using the concrete strategic tactic factory for target services.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	incsat "github.com/glycerine/g3/pkg/sat/sat_solver"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic/portfolio"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func translateStrategicSolver(source *solver.CombinedSolver, manager *ast.ASTManager, params *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result solver.Solver, resultErr error) {
	first, second, err := source.TranslationComponents()
	if err != nil {
		return nil, err
	}
	sourceTactic, ok := first.(*solver.TacticSolver)
	if !ok {
		return nil, fmt.Errorf("api: unsupported strategic solver tactic component")
	}
	logic, proofs, models, cores, err := sourceTactic.TranslationProfile()
	if err != nil {
		return nil, err
	}
	factory, err := portfolio.NewSMTStrategicSolverFactory(logic, config)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, factory.Close())
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	template, err := factory.New(manager, params, proofs, models, cores, logic)
	if err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, template.Close())
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	target, ok := template.(*solver.CombinedSolver)
	if !ok {
		return nil, fmt.Errorf("api: target strategy does not produce a combined solver")
	}
	targetFirst, _, err := target.TranslationComponents()
	if err != nil {
		return nil, err
	}
	targetTactic, ok := targetFirst.(*solver.TacticSolver)
	if !ok {
		return nil, fmt.Errorf("api: unsupported target strategic tactic")
	}
	translatedFirst, err := sourceTactic.TranslateWithTarget(targetTactic, params)
	if err != nil {
		return nil, err
	}
	var translatedSecond solver.Solver
	switch backend := second.(type) {
	case *smtcore.SMTSolver:
		translatedSecond, err = backend.TranslateWithConfig(manager, params, config)
	case *incsat.Solver:
		translatedSecond, err = backend.TranslateWithServices(manager, params, config.ModelEvaluator)
	default:
		err = fmt.Errorf("api: unsupported strategic solver backend")
	}
	if err != nil {
		return nil, errors.Join(err, translatedFirst.Close())
	}
	combined, err := source.TranslateWithComponents(target, translatedFirst, translatedSecond, params)
	if err != nil {
		return nil, errors.Join(err, translatedFirst.Close(), translatedSecond.Close())
	}
	return combined, nil
}
