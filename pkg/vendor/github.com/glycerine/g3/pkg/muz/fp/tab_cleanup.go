package fp

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// tabBackendAttempt owns the query-local implementation of tab::imp.  The
// source destructor retires these objects together; keeping the same owner in
// Go also makes native cleanup refusal retryable without losing later stages.
type tabBackendAttempt struct {
	manager  *ast.ASTManager
	view     *fixedpointParamView
	rw       *rewriter.TheoryRewriter
	coop     *cooperate.Cooperation
	lite     *qe.Lite
	params   *uparams.ParamsContext
	local    uparams.ParamsRef
	localOK  bool
	query    *smtcore.SMTSolver
	sub      *smtcore.SMTSolver
	queryRef ast.ASTRef
}

type tabBackendCleanupFailure struct {
	cause, cleanupErr error
	attempt           *tabBackendAttempt
}

func (f *tabBackendCleanupFailure) Error() string { return f.cause.Error() }
func (f *tabBackendCleanupFailure) Unwrap() error { return f.cause }
func (f *tabBackendCleanupFailure) Cleanup() error {
	f.cleanupErr = f.attempt.close()
	return f.cleanupErr
}
func (f *tabBackendCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.attempt.manager.NumeralService().RuntimeOwner()
}
func (f *tabBackendCleanupFailure) OriginalCause() error { return f.cause }
func (f *tabBackendCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func (a *tabBackendAttempt) close() error {
	// Refuse before any nested close can dismantle part of the backend while a
	// native AST execution is active.  The entire source-shaped owner survives.
	if _, err := a.manager.ProofMode(); err != nil {
		return err
	}
	if a.queryRef != (ast.ASTRef{}) {
		if err := a.manager.DelRef(&a.queryRef); err != nil {
			return err
		}
	}
	if a.sub != nil {
		if err := a.sub.Close(); err != nil {
			return err
		}
		a.sub = nil
	}
	if a.query != nil {
		if err := a.query.Close(); err != nil {
			return err
		}
		a.query = nil
	}
	if a.localOK {
		if err := a.params.Del(&a.local); err != nil {
			return err
		}
		a.localOK = false
	}
	if a.lite != nil {
		if err := a.lite.Close(); err != nil {
			return err
		}
		a.lite = nil
	}
	if a.coop != nil {
		if err := a.coop.Close(); err != nil {
			return err
		}
		a.coop = nil
	}
	if a.rw != nil {
		if err := a.rw.Close(); err != nil {
			return err
		}
		a.rw = nil
	}
	if a.view != nil {
		if err := a.view.Close(); err != nil {
			return err
		}
		a.view = nil
	}
	return nil
}

func (a *tabBackendAttempt) dispose() error {
	if err := a.close(); err != nil {
		return &tabBackendCleanupFailure{cause: err, cleanupErr: err, attempt: a}
	}
	return nil
}

var _ memory.ConstructionFailure = (*tabBackendCleanupFailure)(nil)
