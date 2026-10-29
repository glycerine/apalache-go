package tactic

// Source: src/tactic/tactical.cpp:886-927,1077-1128.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type unaryTacticKind uint8

const (
	unaryFailIfBranching unaryTacticKind = iota
	unaryClean
	unaryRepeat
	unaryTryFor
	unaryUsingParams
	unaryAnnotate
)

type unaryTactic struct {
	self       *unaryTactic
	child      Tactic
	references uint32
	threshold  uint32
	kind       unaryTacticKind
	closed     bool
}

func newUnaryTactic(kind unaryTacticKind, child Tactic, threshold uint32) (*unaryTactic, error) {
	if child == nil {
		return nil, ast.ErrASTHandle
	}
	if err := child.IncRef(); err != nil {
		return nil, err
	}
	t := &unaryTactic{child: child, kind: kind, threshold: threshold}
	t.self = t
	return t, nil
}

func NewFailIfBranchingTactic(child Tactic, threshold uint32) (Tactic, error) {
	return newUnaryTactic(unaryFailIfBranching, child, threshold)
}

func NewCleanTactic(child Tactic) (Tactic, error) {
	return newUnaryTactic(unaryClean, child, 0)
}

func NewRepeatTactic(child Tactic, maximumDepth uint32) (Tactic, error) {
	return newUnaryTactic(unaryRepeat, child, maximumDepth)
}

func NewTryForTactic(child Tactic, milliseconds uint32) (Tactic, error) {
	return newUnaryTactic(unaryTryFor, child, milliseconds)
}

func (t *unaryTactic) open() error {
	if t == nil || t.self != t || t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *unaryTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.references++
	return nil
}
func (t *unaryTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.references == 0 {
		return arena.ErrBounds
	}
	t.references--
	if t.references != 0 {
		return nil
	}
	t.closed = true
	err := t.child.DecRef()
	t.child = nil
	return err
}
func (t *unaryTactic) Apply(g *Goal, out *TacticResult) error {
	if err := t.open(); err != nil {
		return err
	}
	if g == nil || out == nil {
		return ast.ErrASTHandle
	}
	if t.kind == unaryRepeat {
		return t.repeatAt(0, g, out)
	}
	if t.kind == unaryTryFor {
		return t.applyTryFor(g, out)
	}
	if err := t.child.Apply(g, out); err != nil {
		return err
	}
	switch t.kind {
	case unaryFailIfBranching:
		if uint64(len(out.Goals)) > uint64(t.threshold) {
			if err := out.Reset(); err != nil {
				return err
			}
			return NewTacticException("failed-if-branching tactical")
		}
	case unaryClean:
		return t.child.Cleanup()
	}
	return nil
}
func (t *unaryTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.child.UpdateParams(p)
}
func (t *unaryTactic) CollectParamDescrs(p *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.child.CollectParamDescrs(p)
}
func (t *unaryTactic) CollectStatistics() map[string]float64 {
	if t.open() != nil {
		return nil
	}
	return t.child.CollectStatistics()
}
func (t *unaryTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.child.ResetStatistics()
}
func (t *unaryTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.child.Cleanup()
}
func (t *unaryTactic) Reset() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.child.Reset()
}
func (t *unaryTactic) SetLogic(l usymbol.Symbol) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.child.SetLogic(l)
}
func (t *unaryTactic) SetProgressCallback(callback func(uint32)) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.child.SetProgressCallback(callback)
}
func (t *unaryTactic) Translate(m *ast.ASTManager) (Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	child, err := t.child.Translate(m)
	if err != nil {
		return nil, err
	}
	return newUnaryTactic(t.kind, child, t.threshold)
}
