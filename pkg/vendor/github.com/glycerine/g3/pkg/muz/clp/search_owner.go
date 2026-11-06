package clp

// Source: src/muz/clp/clp_context.cpp imp::search expr_ref temporaries and
// imp::m_ground. One attempted rule owns both as a single cleanup unit.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/memory"
)

type ruleAttemptOwner struct {
	manager     *ast.ASTManager
	environment *groundEnvironment
	refs        []ast.ASTRef
}

type ruleAttemptCleanupFailure struct {
	cause      error
	cleanupErr error
	owner      *ruleAttemptOwner
}

func (f *ruleAttemptCleanupFailure) Error() string { return f.cause.Error() }
func (f *ruleAttemptCleanupFailure) Unwrap() error { return f.cause }
func (f *ruleAttemptCleanupFailure) Cleanup() error {
	f.cleanupErr = f.owner.close()
	return f.cleanupErr
}
func (f *ruleAttemptCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *ruleAttemptCleanupFailure) OriginalCause() error { return f.cause }
func (f *ruleAttemptCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (o *ruleAttemptOwner) take(ref ast.ASTRef) {
	o.refs = append(o.refs, ref)
}

func (o *ruleAttemptOwner) close() error {
	for i := len(o.refs) - 1; i >= 0; i-- {
		if o.refs[i] == (ast.ASTRef{}) {
			continue
		}
		if err := o.manager.DelRef(&o.refs[i]); err != nil {
			return err
		}
	}
	o.refs = nil
	return o.environment.close()
}

func (o *ruleAttemptOwner) dispose() error {
	if err := o.close(); err != nil {
		return &ruleAttemptCleanupFailure{cause: err, cleanupErr: err, owner: o}
	}
	return nil
}
