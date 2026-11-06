package bmc

// Source: dl_bmc_engine.cpp linear::get_model and qlinear::get_model.
// The accumulated rule, proof helpers and proof mode outlive a pending step.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/memory"
)

type linearProofOwner struct {
	manager    *ast.ASTManager
	nested     memory.ConstructionFailure
	previous   *base.Rule
	steps      []quantifiedLinearStep
	normalizer *transforms.RuleInlinerPlugin
	scope      *ast.ScopedProofMode
	proof      ast.ASTRef
}

func (o *linearProofOwner) closeLocals() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.previous != nil {
		if err := o.previous.Close(); err != nil {
			return err
		}
		o.previous = nil
	}
	for i := len(o.steps) - 1; i >= 0; i-- {
		if o.steps[i].rule == nil {
			continue
		}
		if err := o.steps[i].rule.Close(); err != nil {
			return err
		}
		o.steps[i].rule = nil
	}
	o.steps = nil
	if o.normalizer != nil {
		if err := o.normalizer.Close(); err != nil {
			return err
		}
		o.normalizer = nil
	}
	if o.scope != nil {
		if err := o.scope.Close(); err != nil {
			return err
		}
		o.scope = nil
	}
	return nil
}

func (o *linearProofOwner) close() error {
	if err := o.closeLocals(); err != nil {
		return err
	}
	if o.proof != (ast.ASTRef{}) {
		return o.manager.DelRef(&o.proof)
	}
	return nil
}

type linearProofCleanupFailure struct {
	owner                       *linearProofOwner
	cause, cleanupErr, reported error
	message                     string
}

func (f *linearProofCleanupFailure) Error() string        { return f.message }
func (f *linearProofCleanupFailure) Unwrap() error        { return f.reported }
func (f *linearProofCleanupFailure) OriginalCause() error { return f.cause }
func (f *linearProofCleanupFailure) CleanupCause() error  { return f.cleanupErr }
func (f *linearProofCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *linearProofCleanupFailure) Cleanup() error {
	f.cleanupErr = f.owner.close()
	return f.cleanupErr
}

func finishLinearProof(o *linearProofOwner, cause error) (ast.ASTRef, error) {
	if cause != nil {
		errors.As(cause, &o.nested)
	}
	cleanup := o.closeLocals()
	if cause == nil && cleanup == nil {
		proof := o.proof
		o.proof = ast.ASTRef{}
		return proof, nil
	}
	if cause == nil {
		cause = cleanup
	}
	if cleanup == nil && o.proof != (ast.ASTRef{}) {
		cleanup = o.manager.DelRef(&o.proof)
	}
	if cleanup == nil {
		return ast.ASTRef{}, cause
	}
	reported := errors.Join(cause, cleanup)
	return ast.ASTRef{}, &linearProofCleanupFailure{
		owner: o, cause: cause, cleanupErr: cleanup,
		reported: reported, message: reported.Error(),
	}
}

var _ memory.ConstructionFailure = (*linearProofCleanupFailure)(nil)
