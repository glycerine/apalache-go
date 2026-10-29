package portfolio

// Source: src/tactic/portfolio/smt_strategic_solver.cpp
// smt_strategic_solver_factory and mk_smt_strategic_solver_factory.

import (
	"io"
	"os"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

type strategicSolverFactory struct {
	logic  string
	config smtcore.SMTSolverConfig
	output io.Writer
	closed bool
}

// NewSMTStrategicSolverFactory creates a source-compatible factory. A nonempty
// logic pins every factory invocation to that logic; an empty logic uses the
// logic supplied to Factory.New.
func NewSMTStrategicSolverFactory(logic string, config smtcore.SMTSolverConfig) (solver.Factory, error) {
	return NewSMTStrategicSolverFactoryWithOutput(logic, config, os.Stdout)
}

// NewSMTStrategicSolverFactoryWithOutput additionally supplies the borrowed
// command-session output used by SAT-backed strategies.
func NewSMTStrategicSolverFactoryWithOutput(logic string, config smtcore.SMTSolverConfig, output io.Writer) (solver.Factory, error) {
	if config.Params == nil || config.Globals == nil || config.Globals.Context != config.Params || output == nil {
		return nil, uparams.ErrParamsOwner
	}
	return &strategicSolverFactory{logic: logic, config: config, output: output}, nil
}

func (f *strategicSolverFactory) New(m *ast.ASTManager, p *uparams.ParamsRef, proofs, models, cores bool, logic string) (solver.Solver, error) {
	if f == nil || f.closed {
		return nil, arena.ErrClosed
	}
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	selected := logic
	if f.logic != "" {
		selected = f.logic
	}
	switch selected {
	case "QF_FD", "SAT":
		enabled, err := m.ProofsEnabled()
		if err != nil {
			return nil, err
		}
		if !enabled {
			return NewFDSolverWithOutput(m, p, f.config.ModelEvaluator, f.output)
		}
	case "QF_AUFLIA":
		return NewQFAUFLIAStrategicSolver(m, p, proofs, models, cores, f.config)
	case "QF_LRA":
		return NewQFLRAStrategicSolver(m, p, proofs, models, cores, f.config)
	case "QF_NRA":
		return NewQFNRAStrategicSolver(m, p, proofs, models, cores, f.config)
	case "QF_FP", "QF_FPBV", "QF_BVFP", "QF_FPLRA":
		return NewQFFPStrategicSolverWithOutput(m, p, proofs, models, cores, selected, f.config, f.output)
	case "QF_NIA":
		return NewQFNIAStrategicSolverWithOutput(m, p, proofs, models, cores, f.config, f.output)
	case "QF_UF":
		return NewQFUFStrategicSolver(m, p, proofs, models, cores, f.config)
	case "QF_IDL":
		return NewQFIDLStrategicSolverWithOutput(m, p, proofs, models, cores, f.config, f.output)
	case "QF_LIA":
		return NewQFLIAStrategicSolverWithOutput(m, p, proofs, models, cores, f.config, f.output)
	case "QF_BV":
		return NewQFBVStrategicSolverWithOutput(m, p, proofs, models, cores, f.config, f.output)
	case "QF_AUFBV":
		return NewQFAUFBVStrategicSolver(m, p, proofs, models, cores, f.config)
	case "QF_ABV":
		return NewQFABVStrategicSolver(m, p, proofs, models, cores, f.config)
	case "QF_UFBV":
		return NewQFUFBVStrategicSolverWithOutput(m, p, proofs, models, cores, f.config, f.output)
	case "UFBV":
		return NewUFBVStrategicSolver(m, p, proofs, models, cores, f.config)
	case "BV":
		return NewBVStrategicSolver(m, p, proofs, models, cores, f.config)
	case "UFNIA":
		return NewUFNIAStrategicSolver(m, p, proofs, models, cores, f.config)
	case "UFLRA":
		return NewUFLRAStrategicSolver(m, p, proofs, models, cores, f.config)
	case "AUFLIA":
		return NewAUFLIAStrategicSolver(m, p, proofs, models, cores, f.config)
	case "AUFLIRA":
		return NewAUFLIRAStrategicSolver(m, p, proofs, models, cores, f.config)
	case "AUFNIRA":
		return NewAUFNIRAStrategicSolver(m, p, proofs, models, cores, f.config)
	case "HORN":
		return NewHornStrategicSolver(m, p, proofs, models, cores, f.config)
	case "ALL":
		return NewALLStrategicSolver(m, p, proofs, models, cores, f.config)
	}
	return nil, z3exception.NewDefaultException("strategic solver logic is not ported: " + selected)
}

func (f *strategicSolverFactory) Close() error {
	if f == nil || f.closed {
		return nil
	}
	f.closed = true
	f.output = nil
	return nil
}

var _ solver.Factory = (*strategicSolverFactory)(nil)
