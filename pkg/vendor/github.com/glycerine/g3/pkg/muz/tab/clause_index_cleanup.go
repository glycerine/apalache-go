package tab

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

type ruleIndexConstructionAttempt struct {
	manager *ast.ASTManager
	nested  memory.ConstructionFailure
	rule    *base.Rule
	clause  *Clause
	index   *RuleIndex
}

func (a *ruleIndexConstructionAttempt) Close() error {
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
	if a.clause != nil {
		if err := a.clause.Close(); err != nil {
			return err
		}
		a.clause = nil
	}
	if a.index != nil {
		if err := a.index.Close(); err != nil {
			return err
		}
		a.index = nil
	}
	return nil
}

type ruleIndexConstructionFailure struct {
	cause, cleanupErr, exposed error
	message                    string
	attempt                    *ruleIndexConstructionAttempt
}

func (f *ruleIndexConstructionFailure) Error() string { return f.message }
func (f *ruleIndexConstructionFailure) Unwrap() error { return f.exposed }
func (f *ruleIndexConstructionFailure) Cleanup() error {
	f.cleanupErr = f.attempt.Close()
	return f.cleanupErr
}
func (f *ruleIndexConstructionFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *ruleIndexConstructionFailure) OriginalCause() error { return f.cause }
func (f *ruleIndexConstructionFailure) CleanupCause() error  { return f.cleanupErr }

func finishRuleIndexConstruction(a *ruleIndexConstructionAttempt, cause error) (*RuleIndex, error) {
	if cause != nil {
		var nested memory.ConstructionFailure
		if errors.As(cause, &nested) {
			a.nested = nested
		}
	}
	result := a.index
	if cause == nil {
		a.index = nil
	}
	cleanup := a.Close()
	if cleanup == nil && cause == nil {
		return result, nil
	}
	if cause == nil {
		a.index = result
		cause = cleanup
	}
	if cleanup == nil {
		return nil, cause
	}
	exposed := errors.Join(cause, cleanup)
	return nil, &ruleIndexConstructionFailure{cause: cause, cleanupErr: cleanup, exposed: exposed, message: exposed.Error(), attempt: a}
}

var _ memory.ConstructionFailure = (*ruleIndexConstructionFailure)(nil)
