package fp

// Source: dl_context.cpp:479-486 publishes each completed mk_rule clause.
// A returning native cleanup error delays that publication, not its ownership.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

type pendingRuleUpdate struct {
	engine      *PropositionalEngine
	clauses     *base.RuleSet
	locals      *contextFormulaAttempt
	publication memory.ConstructionFailure
	failure     *ruleUpdateFailure
	published   bool
	running     bool
	done        bool
}

type ruleUpdateFailure struct {
	update                   *pendingRuleUpdate
	runtime                  *memory.Runtime
	cause, cleanup, reported error
}

var _ memory.ConstructionFailure = (*ruleUpdateFailure)(nil)

func (f *ruleUpdateFailure) Error() string                 { return f.reported.Error() }
func (f *ruleUpdateFailure) Unwrap() error                 { return f.reported }
func (f *ruleUpdateFailure) OriginalCause() error          { return f.cause }
func (f *ruleUpdateFailure) CleanupCause() error           { return f.cleanup }
func (f *ruleUpdateFailure) RuntimeOwner() *memory.Runtime { return f.runtime }
func (f *ruleUpdateFailure) Cleanup() error {
	f.cleanup = f.update.resume()
	return f.cleanup
}

// resume drains lower scopes before touching either rule set. Publication's
// commit checkpoint survives errors in its old-set destructor, so neither
// clauses nor pending-input ranges are shifted a second time on retry.
func (u *pendingRuleUpdate) resume() error {
	if u.done {
		return nil
	}
	if u.running {
		return ast.ErrASTPending
	}
	u.running = true
	defer func() { u.running = false }()
	if u.locals.nested != nil {
		if err := u.locals.nested.Cleanup(); err != nil {
			return err
		}
		u.locals.nested = nil
	}
	if _, err := u.engine.manager.ProofMode(); err != nil {
		return err
	}
	if u.publication != nil {
		if err := u.publication.Cleanup(); err != nil {
			return err
		}
		u.publication = nil
	}
	if !u.published {
		published, err := u.engine.publishRejectedUpdateResult(u.clauses)
		u.published = published
		if err != nil {
			errors.As(err, &u.publication)
			return err
		}
	}
	if err := u.locals.close(); err != nil {
		return err
	}
	u.engine.pendingUpdate = nil
	u.engine, u.clauses, u.locals = nil, nil, nil
	u.done = true
	return nil
}

func (e *PropositionalEngine) deferFailedRuleUpdate(clauses *base.RuleSet, locals *contextFormulaAttempt, cause error) error {
	u := &pendingRuleUpdate{engine: e, clauses: clauses, locals: locals}
	errors.As(cause, &locals.nested)
	f := &ruleUpdateFailure{update: u, runtime: e.manager.NumeralService().RuntimeOwner(), cause: cause, reported: cause}
	u.failure = f
	e.pendingUpdate = u
	if cleanup := u.resume(); cleanup != nil {
		f.cleanup = cleanup
		f.reported = errors.Join(cause, cleanup)
		return f
	}
	return cause
}

func (e *PropositionalEngine) resumePendingRuleUpdate() error {
	u := e.pendingUpdate
	if u == nil {
		return nil
	}
	if err := u.failure.Cleanup(); err != nil {
		return u.failure
	}
	return nil
}
