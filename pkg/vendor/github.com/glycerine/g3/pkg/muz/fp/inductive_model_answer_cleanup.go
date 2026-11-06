package fp

// Source: pdr_manager.cpp and spacer_manager.cpp inductive_property::to_expr
// keep the model_ref alive across model2expr and its temporary expression scopes.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

type inductiveModelAnswerOwner struct {
	manager *ast.ASTManager
	model   *model.Model
	builder *model.Model2ExprBuilder
	output  ast.ASTRef
	nested  memory.ConstructionFailure
}

func (o *inductiveModelAnswerOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.builder != nil {
		if err := o.builder.Close(); err != nil {
			return err
		}
		o.builder = nil
	}
	if o.output != (ast.ASTRef{}) {
		if err := o.manager.DelRef(&o.output); err != nil {
			return err
		}
	}
	if o.model != nil {
		if err := o.model.Close(); err != nil {
			return err
		}
		o.model = nil
	}
	return nil
}

type inductiveModelAnswerCleanupFailure struct {
	owner                    *inductiveModelAnswerOwner
	runtime                  *memory.Runtime
	cause, cleanup, reported error
}

func (f *inductiveModelAnswerCleanupFailure) Error() string                 { return f.reported.Error() }
func (f *inductiveModelAnswerCleanupFailure) Unwrap() error                 { return f.reported }
func (f *inductiveModelAnswerCleanupFailure) OriginalCause() error          { return f.cause }
func (f *inductiveModelAnswerCleanupFailure) CleanupCause() error           { return f.cleanup }
func (f *inductiveModelAnswerCleanupFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *inductiveModelAnswerCleanupFailure) Cleanup() error {
	f.cleanup = f.owner.close()
	return f.cleanup
}

func (o *inductiveModelAnswerOwner) finish(cause error) error {
	if cause != nil {
		errors.As(cause, &o.nested)
	}
	if cleanup := o.close(); cleanup != nil {
		return &inductiveModelAnswerCleanupFailure{
			owner: o, runtime: o.manager.NumeralService().RuntimeOwner(),
			cause: cause, cleanup: cleanup, reported: errors.Join(cause, cleanup),
		}
	}
	return cause
}
