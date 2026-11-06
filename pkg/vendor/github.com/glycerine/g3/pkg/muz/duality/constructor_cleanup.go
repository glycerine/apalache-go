package duality

// Source: duality_wrapper.cpp solver construction and
// duality_dl_interface.cpp duality_data/proof-scope ownership.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	graph "github.com/glycerine/g3/pkg/duality"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type nativeSolverConstruction struct {
	native   solver.Solver
	params   *uparams.ParamsContext
	settings uparams.ParamsRef
	proof    *ast.ScopedProofMode
	runtime  *memory.Runtime
	nested   memory.ConstructionFailure
}

func (c *nativeSolverConstruction) closeLocals() error {
	if c.settings != (uparams.ParamsRef{}) {
		if err := c.params.Del(&c.settings); err != nil {
			return err
		}
	}
	if c.proof != nil {
		if err := c.proof.Close(); err != nil {
			return err
		}
		c.proof = nil
	}
	return nil
}

func (c *nativeSolverConstruction) cleanup() error {
	if c.nested != nil {
		if err := c.nested.Cleanup(); err != nil {
			return err
		}
		c.nested = nil
	}
	// A failed child is destroyed before its enclosing proof scope ends.
	if c.native != nil {
		if err := c.native.Close(); err != nil {
			return err
		}
		c.native = nil
	}
	return c.closeLocals()
}

func (c *nativeSolverConstruction) finish(cause error) (solver.Solver, error) {
	if cause == nil {
		cause = c.closeLocals()
		if cause == nil {
			native := c.native
			c.native = nil
			return native, nil
		}
	}
	errors.As(cause, &c.nested)
	cleanup := c.cleanup()
	if cleanup == nil {
		return nil, cause
	}
	reported := errors.Join(cause, cleanup)
	return nil, &nativeSolverConstructionFailure{owner: c, original: cause, cleanup: cleanup, reported: reported, message: reported.Error()}
}

type nativeSolverConstructionFailure struct {
	owner                       *nativeSolverConstruction
	original, cleanup, reported error
	message                     string
}

func (f *nativeSolverConstructionFailure) Error() string                 { return f.message }
func (f *nativeSolverConstructionFailure) Unwrap() error                 { return f.reported }
func (f *nativeSolverConstructionFailure) OriginalCause() error          { return f.original }
func (f *nativeSolverConstructionFailure) CleanupCause() error           { return f.cleanup }
func (f *nativeSolverConstructionFailure) RuntimeOwner() *memory.Runtime { return f.owner.runtime }
func (f *nativeSolverConstructionFailure) Cleanup() error {
	f.cleanup = f.owner.cleanup()
	return f.cleanup
}

type backendConstruction struct {
	backend *Backend
	config  *graph.Config
	runtime *memory.Runtime
	nested  memory.ConstructionFailure
}

func (c *backendConstruction) closeConfig() error {
	if c.config != nil {
		if err := c.config.Close(); err != nil {
			return err
		}
		c.config = nil
	}
	return nil
}

func (c *backendConstruction) cleanup() error {
	// An unfinished native solver still borrows the backend factory/context.
	if c.nested != nil {
		if err := c.nested.Cleanup(); err != nil {
			return err
		}
		c.nested = nil
	}
	if err := c.closeConfig(); err != nil {
		return err
	}
	if c.backend != nil {
		if err := c.backend.Close(); err != nil {
			return err
		}
		c.backend = nil
	}
	return nil
}

func (c *backendConstruction) finish(cause error) (*Backend, error) {
	if cause == nil {
		cause = c.closeConfig()
		if cause == nil {
			backend := c.backend
			c.backend = nil
			return backend, nil
		}
	}
	errors.As(cause, &c.nested)
	cleanup := c.cleanup()
	if cleanup == nil {
		return nil, cause
	}
	reported := errors.Join(cause, cleanup)
	return nil, &backendConstructionFailure{owner: c, original: cause, cleanup: cleanup, reported: reported, message: reported.Error()}
}

type backendConstructionFailure struct {
	owner                       *backendConstruction
	original, cleanup, reported error
	message                     string
}

func (f *backendConstructionFailure) Error() string                 { return f.message }
func (f *backendConstructionFailure) Unwrap() error                 { return f.reported }
func (f *backendConstructionFailure) OriginalCause() error          { return f.original }
func (f *backendConstructionFailure) CleanupCause() error           { return f.cleanup }
func (f *backendConstructionFailure) RuntimeOwner() *memory.Runtime { return f.owner.runtime }
func (f *backendConstructionFailure) Cleanup() error {
	f.cleanup = f.owner.cleanup()
	return f.cleanup
}

var _ memory.ConstructionFailure = (*nativeSolverConstructionFailure)(nil)
var _ memory.ConstructionFailure = (*backendConstructionFailure)(nil)
