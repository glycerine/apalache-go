package fp

// Source: dl_context.cpp::flush_add_rules and dl_rule.cpp::mk_rule.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type pendingFlushProofOwner struct {
	manager *ast.ASTManager
	scope   *ast.ScopedProofMode
	proof   ast.ASTRef
	nested  memory.ConstructionFailure
}

func (o *pendingFlushProofOwner) close() error {
	if o.nested != nil {
		if err := o.nested.Cleanup(); err != nil {
			return err
		}
		o.nested = nil
	}
	if o.proof != (ast.ASTRef{}) {
		if err := o.manager.DelRef(&o.proof); err != nil {
			return err
		}
	}
	if o.scope != nil {
		if err := o.scope.Close(); err != nil {
			return err
		}
		o.scope = nil
	}
	return nil
}

type pendingFlushProofFailure struct {
	owner                    *pendingFlushProofOwner
	cause, cleanup, reported error
}

func (f *pendingFlushProofFailure) Error() string        { return f.reported.Error() }
func (f *pendingFlushProofFailure) Unwrap() error        { return f.reported }
func (f *pendingFlushProofFailure) OriginalCause() error { return f.cause }
func (f *pendingFlushProofFailure) CleanupCause() error  { return f.cleanup }
func (f *pendingFlushProofFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *pendingFlushProofFailure) Cleanup() error { f.cleanup = f.owner.close(); return f.cleanup }

func (o *pendingFlushProofOwner) finish(cause error) error {
	if cause != nil {
		errors.As(cause, &o.nested)
	}
	if cleanup := o.close(); cleanup != nil {
		return &pendingFlushProofFailure{owner: o, cause: cause, cleanup: cleanup, reported: errors.Join(cause, cleanup)}
	}
	return cause
}

func (e *PropositionalEngine) pendingProofEnabled() (bool, error) {
	enabled := e.forceProofTrace
	if e.params != nil && e.globals != nil {
		view, err := e.fixedpointParamView()
		if err != nil {
			return false, err
		}
		requested, cause := view.Bool("generate_proof_trace", false)
		cause = errors.Join(cause, view.Close())
		if cause != nil {
			return false, cause
		}
		enabled = enabled || requested
	}
	return enabled, nil
}
