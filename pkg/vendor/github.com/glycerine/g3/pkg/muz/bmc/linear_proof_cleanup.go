package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp linear::get_model, where one
// proof step owns its substituted rule, raw resolvent, and proof roots until
// the step is either published or completely discarded.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
	"github.com/glycerine/g3/pkg/util/memory"
)

type linearProofStepOwner struct {
	manager *ast.ASTManager
	scope   *ast.ScopedProofMode
	nested  memory.ConstructionFailure
	refs    []ast.ASTRef
	temps   []*base.Rule
	sub     *rewriter.VarSubstitution
	unifier *transforms.RuleUnifier
	next    *base.Rule
	proof   ast.ASTRef
}

type linearProofStepCleanupFailure struct {
	cause      error
	cleanupErr error
	reported   error
	message    string
	owner      *linearProofStepOwner
}

func (f *linearProofStepCleanupFailure) Error() string {
	return f.message
}
func (f *linearProofStepCleanupFailure) Unwrap() error {
	return f.reported
}
func (f *linearProofStepCleanupFailure) Cleanup() error {
	f.cleanupErr = f.owner.closeAll()
	return f.cleanupErr
}
func (f *linearProofStepCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *linearProofStepCleanupFailure) OriginalCause() error { return f.cause }
func (f *linearProofStepCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (o *linearProofStepOwner) closeLocals() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.scope != nil {
		if err := o.scope.Close(); err != nil {
			return err
		}
		o.scope = nil
	}
	// Match the source lifetime nesting: the variable substitution was
	// constructed after the unifier, so it is destroyed first.
	if o.sub != nil {
		if err := o.sub.Close(); err != nil {
			return err
		}
		o.sub = nil
	}
	if o.unifier != nil {
		if err := o.unifier.Close(); err != nil {
			return err
		}
		o.unifier = nil
	}
	return nil
}

func (o *linearProofStepOwner) closeRefs() error {
	if err := o.closeLocals(); err != nil {
		return err
	}
	if err := closeASTRefs(o.manager, o.refs); err != nil {
		return err
	}
	for i := len(o.temps) - 1; i >= 0; i-- {
		if o.temps[i] == nil {
			continue
		}
		if err := o.temps[i].Close(); err != nil {
			return err
		}
		o.temps[i] = nil
	}
	return nil
}

func (o *linearProofStepOwner) closeOutputs() error {
	if o.next != nil {
		if err := o.next.Close(); err != nil {
			return err
		}
		o.next = nil
	}
	if o.proof != (ast.ASTRef{}) {
		if err := o.manager.DelRef(&o.proof); err != nil {
			return err
		}
	}
	return nil
}

func (o *linearProofStepOwner) closeAll() error {
	if err := o.closeRefs(); err != nil {
		return err
	}
	return o.closeOutputs()
}

// dispose releases temporaries on every path and releases outputs only when
// construction failed. A cleanup refusal transfers all remaining owners into
// one retryable construction failure.
func (o *linearProofStepOwner) dispose(cause error) error {
	if cause != nil {
		// Substitution or normalization can return an unfinished owner that
		// still depends on this step's unifier, expressions and rules.
		errors.As(cause, &o.nested)
	}
	cleanup := o.closeRefs()
	if cleanup == nil && cause != nil {
		cleanup = o.closeOutputs()
	}
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &linearProofStepCleanupFailure{cause: cause, cleanupErr: cleanup, reported: reported, message: reported.Error(), owner: o}
}
