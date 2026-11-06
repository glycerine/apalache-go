package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp nonlinear::bind_vars owns the
// substitution and temporary expressions until the quantified result returns.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

type nonlinearBindingOwner struct {
	manager      *ast.ASTManager
	nested       memory.ConstructionFailure
	substitution *rewriter.VarSubstitution
	refs         []ast.ASTRef
	result       ast.ASTRef
}

func (o *nonlinearBindingOwner) closeLocals() error {
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
	if err := closeASTRefs(o.manager, o.refs); err != nil {
		return err
	}
	o.refs = nil
	return nil
}

func (o *nonlinearBindingOwner) close() error {
	if err := o.closeLocals(); err != nil {
		return err
	}
	if o.result != (ast.ASTRef{}) {
		return o.manager.DelRef(&o.result)
	}
	return nil
}

type nonlinearBindingFailure struct {
	owner                       *nonlinearBindingOwner
	cause, cleanupErr, reported error
	message                     string
}

func (f *nonlinearBindingFailure) Error() string        { return f.message }
func (f *nonlinearBindingFailure) Unwrap() error        { return f.reported }
func (f *nonlinearBindingFailure) OriginalCause() error { return f.cause }
func (f *nonlinearBindingFailure) CleanupCause() error  { return f.cleanupErr }
func (f *nonlinearBindingFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *nonlinearBindingFailure) Cleanup() error {
	f.cleanupErr = f.owner.close()
	return f.cleanupErr
}

func finishNonlinearBinding(o *nonlinearBindingOwner, cause error) (ast.ASTRef, error) {
	if cause != nil {
		errors.As(cause, &o.nested)
	}
	cleanup := o.closeLocals()
	if cause == nil && cleanup == nil {
		result := o.result
		o.result = ast.ASTRef{}
		return result, nil
	}
	if cause == nil {
		cause = cleanup
	}
	if cleanup == nil && o.result != (ast.ASTRef{}) {
		cleanup = o.manager.DelRef(&o.result)
	}
	if cleanup == nil {
		return ast.ASTRef{}, cause
	}
	reported := errors.Join(cause, cleanup)
	return ast.ASTRef{}, &nonlinearBindingFailure{
		owner: o, cause: cause, cleanupErr: cleanup,
		reported: reported, message: reported.Error(),
	}
}

var _ memory.ConstructionFailure = (*nonlinearBindingFailure)(nil)
