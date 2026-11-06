package api

// Source: src/solver/tactic2solver.cpp tactic2solver::translate and
// src/tactic/smtlogics/smt_tactic.cpp tactic translation.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ackermannization"
	"github.com/glycerine/g3/pkg/ast"
	nlsattactic "github.com/glycerine/g3/pkg/nlsat/tactic"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/aig"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/tactic/fpa"
	nlsatsmt "github.com/glycerine/g3/pkg/tactic/nlsat_smt"
	"github.com/glycerine/g3/pkg/tactic/sls"
	"github.com/glycerine/g3/pkg/tactic/smtlogics"
	"github.com/glycerine/g3/pkg/tactic/ufbv"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func translateCallerTacticSolver(source *solver.TacticSolver, target *SMTContext, manager *ast.ASTManager, params *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result solver.Solver, resultErr error) {
	raw, err := source.TranslationTactic()
	if err != nil {
		return nil, err
	}
	targetTactic, err := tactic.TranslateWithContext(raw, manager, config.Params, func(leaf tactic.Tactic) (tactic.Tactic, error) {
		switch sourceTactic := leaf.(type) {
		case *smtcore.SMTTactic:
			return sourceTactic.TranslateWithConfig(manager, config)
		case *smtcore.UnitSubsumptionTactic:
			return sourceTactic.TranslateWithConfig(manager, config)
		case *core.SimplifyTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals)
		case *arith.FactorTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals, config.ModelEvaluator)
		case *arith.AddBoundsTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals)
		case *aig.Tactic:
			return sourceTactic.TranslateWithServices(config.Params, config.ModelEvaluator.Cooperation)
		case *bv.BitBlasterTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals, config.ModelEvaluator.Cooperation)
		case *bv.BV1BlasterTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals, config.ModelEvaluator.Cooperation)
		case *bv.DT2BVTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.ModelEvaluator)
		case *bv.MaxBVSharingTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.ModelEvaluator.Cooperation)
		case *bv.BVSizeReductionTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals, config.ModelEvaluator)
		case *bv.BVBoundCheckTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals, config.ModelEvaluator)
		case *bv.ElimSmallBVTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals, config.ModelEvaluator.Cooperation)
		case *bv.BVArray2UFTactic:
			return sourceTactic.TranslateWithServices(manager, config.ModelEvaluator)
		case *fpa.FPA2BVTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals)
		case *ufbv.MacroTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals, config.ModelEvaluator)
		case *ufbv.RewriterTacticAdapter:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals)
		case *ackermannization.Tactic:
			return sourceTactic.TranslateWithServices(manager, config.ModelEvaluator)
		case *sls.CoreTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals)
		case *qe.LiteTactic:
			return sourceTactic.TranslateWithServices(manager, qe.Config{Context: config.Params, Globals: config.Globals, Cooperation: config.ModelEvaluator.Cooperation, Trace: config.ModelEvaluator.Trace})
		case *qe.QuantifierTactic:
			return sourceTactic.TranslateWithServices(manager, qe.Config{Context: config.Params, Globals: config.Globals, Cooperation: config.ModelEvaluator.Cooperation, Trace: config.ModelEvaluator.Trace})
		case *qe.QESATTactic:
			return sourceTactic.TranslateWithServices(manager, qe.Config{Context: config.Params, Globals: config.Globals, Cooperation: config.ModelEvaluator.Cooperation, Trace: config.ModelEvaluator.Trace})
		case *qe.QSATTactic:
			return sourceTactic.TranslateWithServices(manager, qe.Config{Context: config.Params, Globals: config.Globals, Cooperation: config.ModelEvaluator.Cooperation, Trace: config.ModelEvaluator.Trace})
		case *qe.NLQSATTactic:
			return sourceTactic.TranslateWithServices(manager, qe.Config{Context: config.Params, Globals: config.Globals, Cooperation: config.ModelEvaluator.Cooperation, Trace: config.ModelEvaluator.Trace})
		case *nlsatsmt.NLPurifyTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals, config.ModelEvaluator)
		case *nlsattactic.NLSATTactic:
			return sourceTactic.TranslateWithServices(manager, config.Params, config.Globals, config.ModelEvaluator)
		default:
			if value, matched, err := arith.TranslateContextLeaf(leaf, manager, config.Params, config.Globals, config.ModelEvaluator); matched {
				return value, err
			}
			value, matched, err := core.TranslateContextLeaf(leaf, manager, config.Params, config.Globals, config.ModelEvaluator)
			if matched {
				return value, err
			}
			return nil, fmt.Errorf("api: cross-context translation of tactic %T is not yet supported", leaf)
		}
	}, func(probe tactic.ProbeEvaluator) (tactic.ProbeEvaluator, error) {
		if err := probe.Check(); err != nil {
			return nil, err
		}
		switch probe.(type) {
		case *ackermannization.BoundProbe:
			return ackermannization.NewBoundProbe(manager.Budget())
		case *bv.IsQFBVEqualityProbe:
			return bv.NewIsQFBVEqualityProbe(manager.Budget(), config.Params, config.Globals)
		}
		if translated, matched, err := smtlogics.TranslateContextProbe(probe, manager.Budget()); matched {
			return translated, err
		}
		return arith.TranslateContextProbe(probe, manager.Budget(), config.Params, config.Globals)
	})
	if err != nil {
		return nil, err
	}
	if err = targetTactic.IncRef(); err != nil {
		return nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, targetTactic.DecRef())
		if resultErr != nil && result != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	template, err := target.command.NewSolverFromTactic(targetTactic)
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
	targetSolver, ok := template.(*solver.TacticSolver)
	if !ok {
		return nil, fmt.Errorf("api: unsupported target tactic solver")
	}
	return source.TranslateWithTarget(targetSolver, params)
}
