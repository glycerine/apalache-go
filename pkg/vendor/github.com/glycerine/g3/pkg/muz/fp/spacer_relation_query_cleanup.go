package fp

// The relation-query construction scope owns its temporary expressions and
// unpublished rule until both the construction and temporary cleanup finish.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

type spacerRelationQueryAttempt struct {
	manager *ast.ASTManager
	roots   []ast.ASTRef
	rule    *base.Rule
	nested  memory.ConstructionFailure
}

func (a *spacerRelationQueryAttempt) close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.rule != nil {
		if err := a.rule.Close(); err != nil {
			return err
		}
		a.rule = nil
	}
	if err := releaseQueryReferences(a.manager, a.roots); err != nil {
		return err
	}
	a.roots = nil
	return nil
}

type spacerRelationQueryFailure struct {
	attempt                  *spacerRelationQueryAttempt
	runtime                  *memory.Runtime
	cause, cleanup, reported error
}

func (f *spacerRelationQueryFailure) Error() string                 { return f.reported.Error() }
func (f *spacerRelationQueryFailure) Unwrap() error                 { return f.reported }
func (f *spacerRelationQueryFailure) OriginalCause() error          { return f.cause }
func (f *spacerRelationQueryFailure) CleanupCause() error           { return f.cleanup }
func (f *spacerRelationQueryFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *spacerRelationQueryFailure) Cleanup() error                { f.cleanup = f.attempt.close(); return f.cleanup }

func (a *spacerRelationQueryAttempt) finish(declaration ast.ASTHandle, cause error) (ast.ASTHandle, *base.Rule, error) {
	if cause == nil {
		if cleanup := releaseQueryReferences(a.manager, a.roots); cleanup != nil {
			return ast.ASTHandle{}, nil, &spacerRelationQueryFailure{attempt: a, runtime: a.manager.NumeralService().RuntimeOwner(), cleanup: cleanup, reported: cleanup}
		}
		a.roots = nil
		rule := a.rule
		a.rule = nil
		return declaration, rule, nil
	}
	errors.As(cause, &a.nested)
	if cleanup := a.close(); cleanup != nil {
		return ast.ASTHandle{}, nil, &spacerRelationQueryFailure{attempt: a, runtime: a.manager.NumeralService().RuntimeOwner(), cause: cause, cleanup: cleanup, reported: errors.Join(cause, cleanup)}
	}
	return ast.ASTHandle{}, nil, cause
}
