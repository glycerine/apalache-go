package tab

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

// tabQueryAttempt keeps the query-local proof/search owners reachable when
// native AST cleanup refuses disposal. Cleanup follows their dependency order.
type tabQueryAttempt struct {
	manager    *ast.ASTManager
	properties *base.RuleProperties
	index      *RuleIndex
	goal       *Clause
	search     *tabSearch
	answer     ast.ASTRef
}

func (a *tabQueryAttempt) closeResources() error {
	if a.search != nil {
		if err := a.search.Close(); err != nil {
			return err
		}
		a.search = nil
	}
	if a.goal != nil {
		if err := a.goal.Close(); err != nil {
			return err
		}
		a.goal = nil
	}
	if a.index != nil {
		if err := a.index.Close(); err != nil {
			return err
		}
		a.index = nil
	}
	if a.properties != nil {
		if err := a.properties.Close(); err != nil {
			return err
		}
		a.properties = nil
	}
	return nil
}

func (a *tabQueryAttempt) Close() error {
	if err := a.closeResources(); err != nil {
		return err
	}
	if a.answer != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.answer); err != nil {
			return err
		}
	}
	return nil
}

type tabQueryCleanupFailure struct {
	cause      error
	cleanupErr error
	exposed    error
	message    string
	attempt    *tabQueryAttempt
}

func (f *tabQueryCleanupFailure) Error() string { return f.message }
func (f *tabQueryCleanupFailure) Unwrap() error { return f.exposed }
func (f *tabQueryCleanupFailure) Cleanup() error {
	f.cleanupErr = f.attempt.Close()
	return f.cleanupErr
}
func (f *tabQueryCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *tabQueryCleanupFailure) OriginalCause() error { return f.cause }
func (f *tabQueryCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (a *tabQueryAttempt) finish(answer ast.ASTRef, cause error) (ast.ASTRef, error) {
	cleanup := a.closeResources()
	if cleanup == nil && cause == nil {
		return answer, nil
	}
	if cause == nil {
		cause = cleanup
	}
	if cleanup == nil && answer != (ast.ASTRef{}) {
		a.answer = answer
		answer = ast.ASTRef{}
		cleanup = a.Close()
	}
	if cleanup == nil {
		return ast.ASTRef{}, cause
	}
	if answer != (ast.ASTRef{}) {
		a.answer = answer
	}
	exposed := errors.Join(cause, cleanup)
	return ast.ASTRef{}, &tabQueryCleanupFailure{cause: cause, cleanupErr: cleanup, exposed: exposed, message: exposed.Error(), attempt: a}
}

var _ memory.ConstructionFailure = (*tabQueryCleanupFailure)(nil)

type tabExpressionAttempt struct {
	manager    *ast.ASTManager
	nested     memory.ConstructionFailure
	properties *base.RuleProperties
	queryRule  *base.Rule
	scoped     *base.RuleSet
	compacted  ast.ASTRef
	answer     ast.ASTRef
}

func (a *tabExpressionAttempt) closeResources() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.properties != nil {
		if err := a.properties.Close(); err != nil {
			return err
		}
		a.properties = nil
	}
	if a.queryRule != nil {
		if err := a.queryRule.Close(); err != nil {
			return err
		}
		a.queryRule = nil
	}
	if a.scoped != nil {
		if err := a.scoped.Close(); err != nil {
			return err
		}
		a.scoped = nil
	}
	if a.compacted != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.compacted); err != nil {
			return err
		}
	}
	return nil
}

func (a *tabExpressionAttempt) Close() error {
	if err := a.closeResources(); err != nil {
		return err
	}
	if a.answer != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.answer); err != nil {
			return err
		}
	}
	return nil
}

type tabExpressionCleanupFailure struct {
	cause      error
	cleanupErr error
	attempt    *tabExpressionAttempt
}

func (f *tabExpressionCleanupFailure) Error() string {
	return errors.Join(f.cause, f.cleanupErr).Error()
}
func (f *tabExpressionCleanupFailure) Unwrap() error { return errors.Join(f.cause, f.cleanupErr) }
func (f *tabExpressionCleanupFailure) Cleanup() error {
	f.cleanupErr = f.attempt.Close()
	return f.cleanupErr
}
func (f *tabExpressionCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *tabExpressionCleanupFailure) OriginalCause() error { return f.cause }
func (f *tabExpressionCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (a *tabExpressionAttempt) finish(answer ast.ASTRef, cause error) (ast.ASTRef, error) {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			a.nested = nested
		}
	}
	cleanup := a.closeResources()
	if cleanup == nil && cause == nil {
		return answer, nil
	}
	if cause == nil {
		cause = cleanup
	}
	if cleanup == nil && answer != (ast.ASTRef{}) {
		a.answer = answer
		answer = ast.ASTRef{}
		cleanup = a.Close()
	}
	if cleanup == nil {
		return ast.ASTRef{}, cause
	}
	if answer != (ast.ASTRef{}) {
		a.answer = answer
	}
	return ast.ASTRef{}, &tabExpressionCleanupFailure{cause: cause, cleanupErr: cleanup, attempt: a}
}

var _ memory.ConstructionFailure = (*tabExpressionCleanupFailure)(nil)
