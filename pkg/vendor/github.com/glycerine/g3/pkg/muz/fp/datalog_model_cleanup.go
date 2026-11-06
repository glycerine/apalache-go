package fp

// The finite-table model is an existing Go extension: source rel_context
// inherits engine_base::get_model's empty model. Its model/interpretation
// owners still follow model_core's explicit transfer-on-success contract.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

type datalogModelAttempt struct {
	manager        *ast.ASTManager
	seed, expanded *model.Model
	interpretation *model.FunctionInterpretation
	values         []ast.ASTRef
	nested         memory.ConstructionFailure
}

func (a *datalogModelAttempt) clearValues() error {
	for len(a.values) != 0 {
		i := len(a.values) - 1
		if a.values[i] != (ast.ASTRef{}) {
			if err := a.manager.DelRef(&a.values[i]); err != nil {
				return err
			}
		}
		a.values = a.values[:i]
	}
	return nil
}

func (a *datalogModelAttempt) close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if err := a.clearValues(); err != nil {
		return err
	}
	if a.interpretation != nil {
		if err := a.interpretation.Close(); err != nil {
			return err
		}
		a.interpretation = nil
	}
	if a.expanded != nil {
		if err := a.expanded.Close(); err != nil {
			return err
		}
		if a.seed == a.expanded {
			a.seed = nil
		}
		a.expanded = nil
	}
	if a.seed != nil {
		if err := a.seed.Close(); err != nil {
			return err
		}
		a.seed = nil
	}
	return nil
}

type datalogModelCleanupFailure struct {
	attempt                  *datalogModelAttempt
	runtime                  *memory.Runtime
	cause, cleanup, reported error
}

func (f *datalogModelCleanupFailure) Error() string                 { return f.reported.Error() }
func (f *datalogModelCleanupFailure) Unwrap() error                 { return f.reported }
func (f *datalogModelCleanupFailure) OriginalCause() error          { return f.cause }
func (f *datalogModelCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *datalogModelCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *datalogModelCleanupFailure) Cleanup() error {
	f.cleanup = f.attempt.close()
	return f.cleanup
}

func (a *datalogModelAttempt) fail(cause error) error {
	errors.As(cause, &a.nested)
	if cleanup := a.close(); cleanup != nil {
		return &datalogModelCleanupFailure{
			attempt: a, runtime: a.manager.NumeralService().RuntimeOwner(),
			cause: cause, cleanup: cleanup, reported: errors.Join(cause, cleanup),
		}
	}
	return cause
}
