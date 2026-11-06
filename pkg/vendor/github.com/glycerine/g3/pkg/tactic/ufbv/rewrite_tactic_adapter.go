package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter_tactic.cpp/.h tactic virtual adapter.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// RewriterTacticAdapter exposes the already-ported source goal-ref operation
// through the common tactic result contract.
type RewriterTacticAdapter struct {
	self    *RewriterTacticAdapter
	child   *RewriterTactic
	context *uparams.ParamsContext
	globals *uparams.GlobalParamsService
	refs    uint32
	closed  bool
}

var _ tactic.Tactic = (*RewriterTacticAdapter)(nil)

// RewriterConstructionFailure owns a partially built rewriter when its first
// cleanup attempt cannot complete.
type RewriterConstructionFailure struct {
	owner          *RewriterTactic
	cause, cleanup error
}

func (e *RewriterConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *RewriterConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *RewriterConstructionFailure) OriginalCause() error { return e.cause }
func (e *RewriterConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *RewriterConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *RewriterConstructionFailure) Cleanup() error { return e.owner.Close() }

func releaseFailedRewriter(child *RewriterTactic, cause error) error {
	if child == nil {
		return cause
	}
	if cleanup := child.Close(); cleanup != nil {
		return &RewriterConstructionFailure{owner: child, cause: cause, cleanup: cleanup}
	}
	return cause
}

func NewRewriterTacticAdapter(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*RewriterTacticAdapter, error) {
	child, err := NewRewriterTactic(m, context, globals)
	if err != nil {
		return nil, releaseFailedRewriter(child, err)
	}
	t := &RewriterTacticAdapter{child: child, context: context, globals: globals}
	t.self = t
	return t, nil
}

// TranslateWithServices creates the source's translated rewriter and binds it
// to the target context's parameter owners.
func (t *RewriterTacticAdapter) TranslateWithServices(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if m == nil || context == nil || globals == nil || globals.Context != context {
		return nil, uparams.ErrParamsOwner
	}
	if err := context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	child, err := t.child.Translate(m, context, globals)
	if err != nil {
		return nil, releaseFailedRewriter(child, err)
	}
	child.trace = t.child.trace
	next := &RewriterTacticAdapter{child: child, context: context, globals: globals}
	next.self = next
	return next, nil
}

func (t *RewriterTacticAdapter) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.child == nil {
		return arena.ErrClosed
	}
	return t.child.check()
}
func (t *RewriterTacticAdapter) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *RewriterTacticAdapter) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}
func (t *RewriterTacticAdapter) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.child.Close(); err != nil {
		return err
	}
	t.child, t.closed = nil, true
	return nil
}
func (t *RewriterTacticAdapter) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil {
		return ast.ErrASTHandle
	}
	if err = result.Prepare(goal.Manager()); err != nil {
		return err
	}
	published, err := t.child.Apply(goal)
	if err != nil {
		return err
	}
	if published != goal {
		return errors.Join(ast.ErrASTDomain, published.DecRef())
	}
	if err = result.TakeGoal(goal); err != nil {
		return errors.Join(err, published.DecRef())
	}
	// RewriterTactic publishes the source goal_ref_buffer reference. Transfer
	// that exact incremented reference into the common result owner.
	return nil
}
func (t *RewriterTacticAdapter) UpdateParams(*uparams.ParamsRef) error         { return t.open() }
func (t *RewriterTacticAdapter) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (t *RewriterTacticAdapter) CollectStatistics() map[string]float64         { return nil }
func (t *RewriterTacticAdapter) ResetStatistics() error                        { return t.open() }
func (t *RewriterTacticAdapter) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *RewriterTacticAdapter) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *RewriterTacticAdapter) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.child.Cleanup()
}
func (t *RewriterTacticAdapter) Reset() error { return t.Cleanup() }
func (t *RewriterTacticAdapter) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewRewriterTacticAdapter(m, t.context, t.globals)
}
