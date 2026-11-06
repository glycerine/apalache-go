package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp qlinear::eval_q and
// qlinear::get_model substitution and evaluation lifetimes.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/memory"
)

type quantifiedLinearBindingOwner struct {
	manager      *ast.ASTManager
	nested       memory.ConstructionFailure
	substitution *rewriter.VarSubstitution
	evaluator    *model.ModelEvaluator
	terms        *quantifiedLinear
	outputs      []ast.ASTRef
}

func (o *quantifiedLinearBindingOwner) closeLocals() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.substitution != nil {
		if err := o.substitution.Close(); err != nil {
			return err
		}
		o.substitution = nil
	}
	if o.evaluator != nil {
		if err := o.evaluator.Close(); err != nil {
			return err
		}
		o.evaluator = nil
	}
	if o.terms != nil {
		if err := o.terms.close(); err != nil {
			return err
		}
		o.terms = nil
	}
	return nil
}

func (o *quantifiedLinearBindingOwner) closeOutputs() error {
	if err := closeASTRefs(o.manager, o.outputs); err != nil {
		return err
	}
	o.outputs = nil
	return nil
}

func (o *quantifiedLinearBindingOwner) close() error {
	if err := o.closeLocals(); err != nil {
		return err
	}
	return o.closeOutputs()
}

type quantifiedLinearBindingFailure struct {
	owner                       *quantifiedLinearBindingOwner
	cause, cleanupErr, reported error
	message                     string
}

func (f *quantifiedLinearBindingFailure) Error() string        { return f.message }
func (f *quantifiedLinearBindingFailure) Unwrap() error        { return f.reported }
func (f *quantifiedLinearBindingFailure) OriginalCause() error { return f.cause }
func (f *quantifiedLinearBindingFailure) CleanupCause() error  { return f.cleanupErr }
func (f *quantifiedLinearBindingFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *quantifiedLinearBindingFailure) Cleanup() error {
	f.cleanupErr = f.owner.close()
	return f.cleanupErr
}

func finishQuantifiedLinearBindings(o *quantifiedLinearBindingOwner, cause error) ([]ast.ASTRef, error) {
	if cause != nil {
		errors.As(cause, &o.nested)
	}
	cleanup := o.closeLocals()
	if cause == nil && cleanup == nil {
		outputs := o.outputs
		o.outputs = nil
		return outputs, nil
	}
	if cause == nil {
		cause = cleanup
	}
	if cleanup == nil {
		cleanup = o.closeOutputs()
	}
	if cleanup == nil {
		return nil, cause
	}
	reported := errors.Join(cause, cleanup)
	return nil, &quantifiedLinearBindingFailure{
		owner: o, cause: cause, cleanupErr: cleanup,
		reported: reported, message: reported.Error(),
	}
}

var _ memory.ConstructionFailure = (*quantifiedLinearBindingFailure)(nil)
