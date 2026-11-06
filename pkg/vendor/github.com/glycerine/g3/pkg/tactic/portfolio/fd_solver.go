package portfolio

// Source: src/tactic/portfolio/fd_solver.cpp:28-34; fd_solver.h:26.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	incsat "github.com/glycerine/g3/pkg/sat/sat_solver"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// FDConstructionFailure retains a constructor or destructor stop from the
// concrete wrapper chain. Cleanup explicitly retries/acknowledges that stop.
type FDConstructionFailure struct {
	current        solver.Solver
	nested         memory.ConstructionFailure
	cause, cleanup error
	runtime        *memory.Runtime
}

func (e *FDConstructionFailure) Error() string                 { return e.Unwrap().Error() }
func (e *FDConstructionFailure) Unwrap() error                 { return errors.Join(e.cause, e.cleanup) }
func (e *FDConstructionFailure) OriginalCause() error          { return e.cause }
func (e *FDConstructionFailure) CleanupCause() error           { return e.cleanup }
func (e *FDConstructionFailure) RuntimeOwner() *memory.Runtime { return e.runtime }

// CleanupPending lets enclosing owners keep manager/services alive until the
// explicit cleanup receipt has been acknowledged.
func (e *FDConstructionFailure) CleanupPending() bool { return e.current != nil || e.nested != nil }
func (e *FDConstructionFailure) Cleanup() error {
	if e.nested != nil {
		if err := e.nested.Cleanup(); err != nil {
			// Wrapper Close preserves the SAT core's observed destructor stop.
			// This explicit Cleanup acknowledges that concrete receipt before retrying
			// its owning wrapper; ordinary construction/Close never suppresses it.
			var receipt *incsat.CleanupFailure
			if !errors.As(err, &receipt) {
				return err
			}
			if err = receipt.Cleanup(); err != nil {
				return err
			}
			if err = e.nested.Cleanup(); err != nil {
				return err
			}
		}
		e.nested = nil
	}
	if e.current != nil {
		if err := e.current.Close(); err != nil {
			return err
		}
		e.current = nil
	}
	return nil
}

// NewFDSolver creates the source stack: bounded-int -> PB -> enum -> SAT.
func NewFDSolver(m *ast.ASTManager, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (solver.Solver, error) {
	return NewFDSolverWithOutput(m, p, services, nil)
}
func NewFDSolverWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, services model.ModelEvaluatorConfig, output io.Writer) (result solver.Solver, err error) {
	var current solver.Solver
	defer func() {
		if err == nil {
			return
		}
		var child memory.ConstructionFailure
		if errors.As(err, &child) {
			result = nil
			err = &FDConstructionFailure{current: current, nested: child, cause: err, cleanup: child.CleanupCause(), runtime: services.Memory}
			return
		}
		if current != nil {
			if cleanup := current.Close(); cleanup != nil {
				var nested memory.ConstructionFailure
				errors.As(cleanup, &nested)
				result = nil
				err = &FDConstructionFailure{current: current, nested: nested, cause: err, cleanup: cleanup, runtime: services.Memory}
			}
		}
	}()
	base, e := incsat.NewIncrementalSolverWithConfig(m, p, incsat.IncrementalSolverConfig{Services: services, Output: output})
	if base != nil {
		current = base
	}
	if e != nil {
		return nil, e
	}
	enums, e := NewEnum2BVSolver(m, services.Context, p, current, services)
	if e != nil {
		return nil, e
	}
	current = enums
	pseudo, e := NewPB2BVSolver(m, services.Context, p, current)
	if e != nil {
		return nil, e
	}
	current = pseudo
	bounded, e := NewBoundedInt2BVSolver(m, services.Context, p, current, services)
	if e != nil {
		return nil, e
	}
	return bounded, nil
}
