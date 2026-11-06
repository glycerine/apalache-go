package spacer

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

type spacerTraceCleanupAttempt struct {
	manager *ast.ASTManager
	facts   []*ReachFact
	rules   []*base.Rule
}

func (a *spacerTraceCleanupAttempt) closeFacts() error {
	for len(a.facts) != 0 {
		last := len(a.facts) - 1
		if err := a.facts[last].Close(); err != nil {
			return err
		}
		a.facts[last] = nil
		a.facts = a.facts[:last]
	}
	return nil
}

func (a *spacerTraceCleanupAttempt) close() error {
	if err := a.closeFacts(); err != nil {
		return err
	}
	for len(a.rules) != 0 {
		last := len(a.rules) - 1
		if err := a.rules[last].Close(); err != nil {
			return err
		}
		a.rules[last] = nil
		a.rules = a.rules[:last]
	}
	return nil
}

type spacerTraceCleanupFailure struct {
	attempt           *spacerTraceCleanupAttempt
	cause, cleanupErr error
	reported          error
	reportText        string
}

func (f *spacerTraceCleanupFailure) Error() string { return f.reportText }
func (f *spacerTraceCleanupFailure) Unwrap() error { return f.reported }
func (f *spacerTraceCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *spacerTraceCleanupFailure) OriginalCause() error { return f.cause }
func (f *spacerTraceCleanupFailure) CleanupCause() error  { return f.cleanupErr }
func (f *spacerTraceCleanupFailure) Cleanup() error {
	f.cleanupErr = f.attempt.close()
	return f.cleanupErr
}

func (a *spacerTraceCleanupAttempt) finish(cause error) error {
	cleanup := a.close()
	if cleanup == nil {
		return cause
	}
	if cause == nil {
		cause = cleanup
	}
	reported := errors.Join(cause, cleanup)
	return &spacerTraceCleanupFailure{attempt: a, cause: cause, cleanupErr: cleanup, reported: reported, reportText: reported.Error()}
}

var _ memory.ConstructionFailure = (*spacerTraceCleanupFailure)(nil)
