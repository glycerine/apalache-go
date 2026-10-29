package portfolio

// Source: src/tactic/portfolio/smt_strategic_solver.cpp retained QF_AUFLIA and
// QF_LRA/QF_UF/QF_IDL/QF_LIA/QF_BV/QF_AUFBV/QF_ABV/QF_UFBV/QF_FP and
// UFNIA/UFLRA/AUFLIA/AUFLIRA/AUFNIRA branches and
// smt_strategic_solver_factory::operator().

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	muzfp "github.com/glycerine/g3/pkg/muz/fp"
	incsat "github.com/glycerine/g3/pkg/sat/sat_solver"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/tactic/smtlogics"
	ufbvtactic "github.com/glycerine/g3/pkg/tactic/ufbv"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// NewQFAUFLIAStrategicSolver connects the source tactic solver and incremental
// SMT solver through combined_solver's ordinary mode/fallback policy. Other
// logic strategies remain separate ports of the source factory dispatcher.
func NewQFAUFLIAStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (result solver.Solver, err error) {
	strategy, err := smtlogics.NewQFAUFLIATactic(m, p, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "QF_AUFLIA", p, proofs, models, cores, config)
}

// NewALLStrategicSolver retains the default strategy's theory-neutral
// simplification, propagation, equation solving and unconstrained-variable
// elimination path. The direct ALL SMT solver remains the incremental backend,
// so combined_solver owns the same nonincremental/incremental transition and
// unknown fallback as the source factory.
func NewALLStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (result solver.Solver, err error) {
	var tacticOwners []tactic.Tactic
	var probeOwners []tactic.ProbeEvaluator
	defer func() {
		for i := len(tacticOwners) - 1; i >= 0; i-- {
			err = errors.Join(err, tacticOwners[i].DecRef())
		}
		for i := len(probeOwners) - 1; i >= 0; i-- {
			err = errors.Join(err, probeOwners[i].DecRef())
		}
	}()
	keepTactic := func(value tactic.Tactic, cause error) (tactic.Tactic, error) {
		if cause != nil {
			return nil, cause
		}
		if cause = value.IncRef(); cause != nil {
			return nil, cause
		}
		tacticOwners = append(tacticOwners, value)
		return value, nil
	}
	keepProbe := func(value tactic.ProbeEvaluator, cause error) (tactic.ProbeEvaluator, error) {
		if cause != nil {
			return nil, cause
		}
		if cause = value.IncRef(); cause != nil {
			return nil, cause
		}
		probeOwners = append(probeOwners, value)
		return value, nil
	}
	qfAUFLIA, err := smtlogics.NewQFAUFLIATactic(m, p, config)
	qfAUFLIA, err = keepTactic(qfAUFLIA, err)
	if err != nil {
		return nil, err
	}
	qfBV, err := smtlogics.NewQFBVTactic(m, p, config)
	qfBV, err = keepTactic(qfBV, err)
	if err != nil {
		return nil, err
	}
	qfAUFBV, err := smtlogics.NewQFAUFBVTactic(m, p, config)
	qfAUFBV, err = keepTactic(qfAUFBV, err)
	if err != nil {
		return nil, err
	}
	qfLIA, err := smtlogics.NewQFLIATactic(m, p, config)
	qfLIA, err = keepTactic(qfLIA, err)
	if err != nil {
		return nil, err
	}
	qfLRA, err := smtlogics.NewQFLRATactic(m, p, config)
	qfLRA, err = keepTactic(qfLRA, err)
	if err != nil {
		return nil, err
	}
	qfNIA, err := smtlogics.NewQFNIATactic(m, p, config)
	qfNIA, err = keepTactic(qfNIA, err)
	if err != nil {
		return nil, err
	}
	qfNRA, err := smtlogics.NewQFNRATactic(m, p, config)
	qfNRA, err = keepTactic(qfNRA, err)
	if err != nil {
		return nil, err
	}
	qfFP, err := smtlogics.NewQFFPTactic(m, p, config)
	qfFP, err = keepTactic(qfFP, err)
	if err != nil {
		return nil, err
	}
	qfFPLRA, err := smtlogics.NewQFFPLRATactic(m, p, config)
	qfFPLRA, err = keepTactic(qfFPLRA, err)
	if err != nil {
		return nil, err
	}
	ordinaryValue, err := smtcore.NewSMTTactic(m, p, config)
	ordinary, err := keepTactic(ordinaryValue, err)
	if err != nil {
		return nil, err
	}
	probe, err := newALLAsArrayProbe(m)
	asArray, err := keepProbe(probe, err)
	if err != nil {
		return nil, err
	}
	lira, err := smtlogics.NewQuantifierFreeLIRATactic(m, p, config)
	lira, err = keepTactic(lira, err)
	if err != nil {
		return nil, err
	}
	liraFragment, err := keepProbe(arith.NewArithmeticFragmentProbe(m.Budget(), config.Params, config.Globals, arith.FragmentLIRA))
	if err != nil {
		return nil, err
	}
	qfLIAProbe, err := keepProbe(arith.NewArithmeticFragmentProbe(m.Budget(), config.Params, config.Globals, arith.FragmentQFLIA))
	if err != nil {
		return nil, err
	}
	qfAUFLIAProbe, err := keepProbe(arith.NewArithmeticFragmentProbe(m.Budget(), config.Params, config.Globals, arith.FragmentQFAUFLIA))
	if err != nil {
		return nil, err
	}
	qfLRAProbe, err := keepProbe(arith.NewArithmeticFragmentProbe(m.Budget(), config.Params, config.Globals, arith.FragmentQFLRA))
	if err != nil {
		return nil, err
	}
	qfNIAProbe, err := keepProbe(arith.NewArithmeticFragmentProbe(m.Budget(), config.Params, config.Globals, arith.FragmentQFNIA))
	if err != nil {
		return nil, err
	}
	qfNRAProbe, err := keepProbe(arith.NewArithmeticFragmentProbe(m.Budget(), config.Params, config.Globals, arith.FragmentQFNRA))
	if err != nil {
		return nil, err
	}
	qfFPProbe, err := keepProbe(smtlogics.NewIsQFFPProbe(m.Budget()))
	if err != nil {
		return nil, err
	}
	qfFPLRAProbe, err := keepProbe(smtlogics.NewIsQFFPLRAProbe(m.Budget()))
	if err != nil {
		return nil, err
	}
	qfAUFBVProbe, err := keepProbe(tactic.NewProbe(m.Budget(), tactic.ProbeIsQFArrayUFBitVector, 0))
	if err != nil {
		return nil, err
	}
	qfBVProbe, err := keepProbe(tactic.NewProbe(m.Budget(), tactic.ProbeIsQFBitVector, 0))
	if err != nil {
		return nil, err
	}
	qfSingleSort, err := keepProbe(tactic.NewBinaryProbe(m.Budget(), tactic.ProbeOr, qfLIAProbe, qfLRAProbe))
	if err != nil {
		return nil, err
	}
	mixedArithmetic, err := keepProbe(tactic.NewNotProbe(m.Budget(), qfSingleSort))
	if err != nil {
		return nil, err
	}
	liraMixed, err := keepProbe(tactic.NewBinaryProbe(m.Budget(), tactic.ProbeAnd, liraFragment, mixedArithmetic))
	if err != nil {
		return nil, err
	}
	hasQuantifier, err := keepProbe(tactic.NewProbe(m.Budget(), tactic.ProbeHasQuantifier, 0))
	if err != nil {
		return nil, err
	}
	noQuantifier, err := keepProbe(tactic.NewNotProbe(m.Budget(), hasQuantifier))
	if err != nil {
		return nil, err
	}
	qfLIRA, err := keepProbe(tactic.NewBinaryProbe(m.Budget(), tactic.ProbeAnd, liraMixed, noQuantifier))
	if err != nil {
		return nil, err
	}
	route, err := tactic.NewCondTactic(qfLIRA, lira, ordinary)
	route, err = keepTactic(route, err)
	if err != nil {
		return nil, err
	}
	for _, branch := range []struct {
		probe  tactic.ProbeEvaluator
		tactic tactic.Tactic
	}{
		{qfFPLRAProbe, qfFPLRA},
		{qfFPProbe, qfFP},
		{qfNIAProbe, qfNIA},
		{qfNRAProbe, qfNRA},
		{qfLRAProbe, qfLRA},
		{qfAUFLIAProbe, qfAUFLIA},
		{qfLIAProbe, qfLIA},
		{qfAUFBVProbe, qfAUFBV},
		{qfBVProbe, qfBV},
	} {
		if err != nil {
			return nil, err
		}
		route, err = tactic.NewCondTactic(branch.probe, branch.tactic, route)
		route, err = keepTactic(route, err)
		if err != nil {
			return nil, err
		}
	}
	route, err = tactic.NewCondTactic(asArray, qfAUFLIA, route)
	route, err = keepTactic(route, err)
	if err != nil {
		return nil, err
	}
	simplify, err := core.NewSimplifyTactic(m, config.Params, config.Globals, p)
	if err != nil {
		return nil, err
	}
	simplifier, err := keepTactic(simplify, nil)
	if err != nil {
		return nil, err
	}
	strategy, err := tactic.NewAndThenTactic(simplifier, route)
	if err != nil {
		return nil, err
	}
	result, err = newStrategicSolver(m, strategy, "ALL", p, proofs, models, cores, config)
	return result, err
}

func NewQFLRAStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	strategy, err := smtlogics.NewQFLRATactic(m, p, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "QF_LRA", p, proofs, models, cores, config)
}

func NewQFNRAStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	strategy, err := smtlogics.NewQFNRATactic(m, p, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "QF_NRA", p, proofs, models, cores, config)
}

func NewQFFPStrategicSolverWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, logic string, config smtcore.SMTSolverConfig, output io.Writer) (solver.Solver, error) {
	var strategy tactic.Tactic
	var err error
	if logic == "QF_FPBV" || logic == "QF_BVFP" {
		strategy, err = smtlogics.NewQFFPBVTacticWithOutput(m, p, config, output)
	} else {
		strategy, err = smtlogics.NewQFFPTacticWithOutput(m, p, config, output)
	}
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, logic, p, proofs, models, cores, config)
}

func NewQFNIAStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	return NewQFNIAStrategicSolverWithOutput(m, p, proofs, models, cores, config, os.Stdout)
}

func NewQFNIAStrategicSolverWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig, output io.Writer) (solver.Solver, error) {
	strategy, err := smtlogics.NewQFNIATacticWithOutput(m, p, config, output)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "QF_NIA", p, proofs, models, cores, config)
}

func NewQFUFStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	strategy, err := smtlogics.NewQFUFTactic(m, p, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "QF_UF", p, proofs, models, cores, config)
}

func NewQFIDLStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	return NewQFIDLStrategicSolverWithOutput(m, p, proofs, models, cores, config, os.Stdout)
}

func NewQFIDLStrategicSolverWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig, output io.Writer) (solver.Solver, error) {
	strategy, err := smtlogics.NewQFIDLTacticWithOutput(m, p, config, output)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "QF_IDL", p, proofs, models, cores, config)
}

func NewQFLIAStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	return NewQFLIAStrategicSolverWithOutput(m, p, proofs, models, cores, config, os.Stdout)
}

func NewQFLIAStrategicSolverWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig, output io.Writer) (solver.Solver, error) {
	strategy, err := smtlogics.NewQFLIATacticWithOutput(m, p, config, output)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "QF_LIA", p, proofs, models, cores, config)
}

func NewQFBVStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	return NewQFBVStrategicSolverWithOutput(m, p, proofs, models, cores, config, os.Stdout)
}

func NewQFBVStrategicSolverWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig, output io.Writer) (solver.Solver, error) {
	strategy, err := smtlogics.NewQFBVTacticWithOutput(m, p, config, output)
	if err != nil {
		return nil, err
	}
	var targetServices func(*ast.ASTManager) (model.ModelEvaluatorConfig, error)
	if config.TargetConfig != nil {
		targetServices = func(target *ast.ASTManager) (model.ModelEvaluatorConfig, error) {
			targetConfig, targetErr := config.TargetConfig(target)
			return targetConfig.ModelEvaluator, targetErr
		}
	}
	backend, err := incsat.NewIncrementalSolverWithConfig(m, p, incsat.IncrementalSolverConfig{
		Services:       config.ModelEvaluator,
		TargetServices: targetServices,
		Output:         output,
	})
	if err != nil {
		return nil, err
	}
	return newStrategicSolverWithBackend(m, strategy, backend, "QF_BV", p, proofs, models, cores, config)
}

func NewQFAUFBVStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	strategy, err := smtlogics.NewQFAUFBVTactic(m, p, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "QF_AUFBV", p, proofs, models, cores, config)
}

func NewQFABVStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	strategy, err := smtlogics.NewQFAUFBVTactic(m, p, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "QF_ABV", p, proofs, models, cores, config)
}

func NewQFUFBVStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	return NewQFUFBVStrategicSolverWithOutput(m, p, proofs, models, cores, config, os.Stdout)
}

func NewQFUFBVStrategicSolverWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig, output io.Writer) (solver.Solver, error) {
	strategy, err := smtlogics.NewQFUFBVTacticWithOutput(m, p, config, output)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "QF_UFBV", p, proofs, models, cores, config)
}

func NewUFBVStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	strategy, err := ufbvtactic.NewUFBVTactic(m, p, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "UFBV", p, proofs, models, cores, config)
}

func NewBVStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	strategy, err := ufbvtactic.NewUFBVTactic(m, p, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "BV", p, proofs, models, cores, config)
}

func NewUFLRAStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	strategy, err := smtlogics.NewUFLRATactic(m, p, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "UFLRA", p, proofs, models, cores, config)
}

func NewUFNIAStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	strategy, err := smtlogics.NewUFNIATactic(m, p, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "UFNIA", p, proofs, models, cores, config)
}

func NewAUFLIAStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	strategy, err := smtlogics.NewAUFLIATactic(m, p, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "AUFLIA", p, proofs, models, cores, config)
}

func NewAUFLIRAStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	strategy, err := smtlogics.NewAUFLIRATactic(m, p, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "AUFLIRA", p, proofs, models, cores, config)
}

func NewAUFNIRAStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	strategy, err := smtlogics.NewAUFNIRATactic(m, p, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "AUFNIRA", p, proofs, models, cores, config)
}

// NewHornStrategicSolver retains the source HORN tactic-first combined solver.
// The ordinary SMT solver remains the incremental and unknown fallback.
func NewHornStrategicSolver(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (solver.Solver, error) {
	strategy, err := muzfp.NewHornTactic(m, config.Params, config.Globals)
	if err != nil {
		return nil, err
	}
	return newStrategicSolver(m, strategy, "HORN", p, proofs, models, cores, config)
}

func newStrategicSolver(m *ast.ASTManager, strategy tactic.Tactic, logic string, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (result solver.Solver, err error) {
	second, err := smtcore.NewSMTSolver(m, p, logic, config)
	if err != nil {
		return nil, err
	}
	return newStrategicSolverWithBackend(m, strategy, second, logic, p, proofs, models, cores, config)
}

func newStrategicSolverWithBackend(m *ast.ASTManager, strategy tactic.Tactic, second solver.Solver, logic string, p *uparams.ParamsRef, proofs, models, cores bool, config smtcore.SMTSolverConfig) (result solver.Solver, err error) {
	if err = strategy.IncRef(); err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, strategy.DecRef()) }()
	first, err := solver.NewTacticSolver(m, strategy, p, proofs, models, cores, logic, solver.TacticSolverConfig{
		Params: config.Params,
		EvaluatorFactory: func(md *model.ModelCore) (solver.ConsequenceEvaluator, error) {
			return model.NewModelEvaluator(md, config.ModelEvaluator)
		},
	})
	if err != nil {
		return nil, err
	}
	if err = first.IncRef(); err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, first.DecRef()) }()
	if err = second.IncRef(); err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, second.DecRef()) }()
	combined, err := solver.NewCombinedSolver(first, second, p, solver.CombinedSolverConfig{Params: config.Params, Globals: config.Globals})
	if err != nil {
		return nil, err
	}
	return combined, nil
}
