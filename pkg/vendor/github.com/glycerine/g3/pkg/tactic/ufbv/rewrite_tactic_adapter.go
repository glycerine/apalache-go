package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter_tactic.cpp/.h tactic virtual adapter.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
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

func NewRewriterTacticAdapter(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*RewriterTacticAdapter, error) {
	child, err := NewRewriterTactic(m, context, globals)
	if err != nil {
		return nil, err
	}
	t := &RewriterTacticAdapter{child: child, context: context, globals: globals}
	t.self = t
	return t, nil
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
