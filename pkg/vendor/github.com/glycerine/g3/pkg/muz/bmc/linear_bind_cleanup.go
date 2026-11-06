package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp bmc::linear::mk_rule_vars and
// bmc::linear::get_model, whose expr_ref_vector substitutions own both the
// symbolic bindings and the values extracted from the winning model.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type linearBindingOwner struct {
	manager *ast.ASTManager
	inputs  []ast.ASTRef
	outputs []ast.ASTRef
}

type linearBindingCleanupFailure struct {
	cause      error
	cleanupErr error
	owner      *linearBindingOwner
}

func (f *linearBindingCleanupFailure) Error() string {
	return errors.Join(f.cause, f.cleanupErr).Error()
}
func (f *linearBindingCleanupFailure) Unwrap() error {
	return errors.Join(f.cause, f.cleanupErr)
}
func (f *linearBindingCleanupFailure) Cleanup() error {
	f.cleanupErr = f.owner.closeAll()
	return f.cleanupErr
}
func (f *linearBindingCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *linearBindingCleanupFailure) OriginalCause() error { return f.cause }
func (f *linearBindingCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (o *linearBindingOwner) closeInputs() error {
	return closeASTRefs(o.manager, o.inputs)
}

func (o *linearBindingOwner) closeOutputs() error {
	return closeASTRefs(o.manager, o.outputs)
}

func (o *linearBindingOwner) closeAll() error {
	if err := o.closeInputs(); err != nil {
		return err
	}
	return o.closeOutputs()
}

// dispose always releases consumed symbolic inputs. Outputs are published only
// when both construction and input cleanup succeed.
func (o *linearBindingOwner) dispose(cause error) error {
	cleanup := o.closeInputs()
	if cleanup == nil && cause != nil {
		cleanup = o.closeOutputs()
	}
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
	}
	return &linearBindingCleanupFailure{cause: cause, cleanupErr: cleanup, owner: o}
}
