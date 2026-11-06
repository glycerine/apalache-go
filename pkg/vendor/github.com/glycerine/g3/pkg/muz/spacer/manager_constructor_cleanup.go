package spacer

// Source: spacer_context.cpp context construction and the manager members in
// spacer_manager.cpp. Failed construction must finish its local destruction
// before releasing the enclosing owner.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type spacerManagerConstruction struct {
	manager *Manager
	runtime *memory.Runtime
	nested  memory.ConstructionFailure
}

func (c *spacerManagerConstruction) close() error {
	if c.nested != nil {
		if err := c.nested.Cleanup(); err != nil {
			return err
		}
		c.nested = nil
	}
	if c.manager != nil {
		if err := c.manager.Close(); err != nil {
			return err
		}
		c.manager = nil
	}
	return nil
}

func (c *spacerManagerConstruction) finish(cause error) (*Manager, error) {
	if cause == nil {
		result := c.manager
		c.manager = nil
		return result, nil
	}
	errors.As(cause, &c.nested)
	if cleanup := c.close(); cleanup != nil {
		reported := errors.Join(cause, cleanup)
		return nil, &spacerManagerConstructionFailure{owner: c, cause: cause, cleanup: cleanup, reported: reported, message: reported.Error()}
	}
	return nil, cause
}

type spacerManagerConstructionFailure struct {
	owner                    *spacerManagerConstruction
	cause, cleanup, reported error
	message                  string
}

func (f *spacerManagerConstructionFailure) Error() string                 { return f.message }
func (f *spacerManagerConstructionFailure) Unwrap() error                 { return f.reported }
func (f *spacerManagerConstructionFailure) OriginalCause() error          { return f.cause }
func (f *spacerManagerConstructionFailure) CleanupCause() error           { return f.cleanup }
func (f *spacerManagerConstructionFailure) RuntimeOwner() *memory.Runtime { return f.owner.runtime }
func (f *spacerManagerConstructionFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

// The solver parameter copy has its own lifetime before NewManager can adopt
// it. A numerical decrement may consume the reference before cleanup stops.
type spacerSolverParamsConstruction struct {
	context *uparams.ParamsContext
	params  uparams.ParamsRef
	receipt uparams.ParamsDecrementReceipt
	runtime *memory.Runtime
	nested  memory.ConstructionFailure
}

func (c *spacerSolverParamsConstruction) close() error {
	if c.nested != nil {
		if err := c.nested.Cleanup(); err != nil {
			return err
		}
		c.nested = nil
	}
	if c.receipt.Invocation() != 0 {
		if err := c.context.CompleteDecrement(&c.params, c.receipt); err != nil {
			return err
		}
		c.receipt = uparams.ParamsDecrementReceipt{}
		return nil
	}
	if c.params.Slot == 0 {
		return nil
	}
	if err := c.context.DelTracked(&c.params, &c.receipt); err != nil {
		return err
	}
	c.receipt = uparams.ParamsDecrementReceipt{}
	return nil
}

func (c *spacerSolverParamsConstruction) finish(cause error) (uparams.ParamsRef, error) {
	if cause == nil {
		result := c.params
		c.params = uparams.ParamsRef{}
		return result, nil
	}
	errors.As(cause, &c.nested)
	if cleanup := c.close(); cleanup != nil {
		reported := errors.Join(cause, cleanup)
		return uparams.ParamsRef{}, &spacerSolverParamsConstructionFailure{owner: c, cause: cause, cleanup: cleanup, reported: reported, message: reported.Error()}
	}
	return uparams.ParamsRef{}, cause
}

type spacerSolverParamsConstructionFailure struct {
	owner                    *spacerSolverParamsConstruction
	cause, cleanup, reported error
	message                  string
}

func (f *spacerSolverParamsConstructionFailure) Error() string        { return f.message }
func (f *spacerSolverParamsConstructionFailure) Unwrap() error        { return f.reported }
func (f *spacerSolverParamsConstructionFailure) OriginalCause() error { return f.cause }
func (f *spacerSolverParamsConstructionFailure) CleanupCause() error  { return f.cleanup }
func (f *spacerSolverParamsConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.runtime
}
func (f *spacerSolverParamsConstructionFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}
