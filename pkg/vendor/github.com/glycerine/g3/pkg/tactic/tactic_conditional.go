package tactic

// Source: src/tactic/tactical.cpp:1225-1387; src/tactic/tactical.h:71-88.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type conditionalTacticKind uint8

const (
	conditionalChoose conditionalTacticKind = iota
	conditionalFailIf
	conditionalFailIfNot
	conditionalIfNoProofs
	conditionalIfNoCores
	conditionalIfNoModels
)

type conditionalTactic struct {
	self       *conditionalTactic
	probe      ProbeEvaluator
	thenTactic Tactic
	elseTactic Tactic
	references uint32
	kind       conditionalTacticKind
	closed     bool
}

func newConditionalTactic(kind conditionalTacticKind, probe ProbeEvaluator, thenTactic, elseTactic Tactic) (t *conditionalTactic, err error) {
	if thenTactic == nil || kind <= conditionalFailIfNot && probe == nil || kind == conditionalChoose && elseTactic == nil {
		return nil, ast.ErrASTHandle
	}
	t = &conditionalTactic{probe: probe, thenTactic: thenTactic, elseTactic: elseTactic, kind: kind}
	t.self = t
	if probe != nil {
		if err = probe.IncRef(); err != nil {
			return nil, err
		}
	}
	if err = thenTactic.IncRef(); err != nil {
		if probe != nil {
			_ = probe.DecRef()
		}
		return nil, err
	}
	if elseTactic != nil {
		if err = elseTactic.IncRef(); err != nil {
			_ = thenTactic.DecRef()
			if probe != nil {
				_ = probe.DecRef()
			}
			return nil, err
		}
	}
	return t, nil
}

func NewCondTactic(p ProbeEvaluator, thenTactic, elseTactic Tactic) (Tactic, error) {
	return newConditionalTactic(conditionalChoose, p, thenTactic, elseTactic)
}

func NewWhenTactic(p ProbeEvaluator, child Tactic) (Tactic, error) {
	return newConditionalTactic(conditionalChoose, p, child, NewSkipTactic())
}

func NewFailIfTactic(p ProbeEvaluator) (Tactic, error) {
	return newConditionalTactic(conditionalFailIf, p, NewSkipTactic(), nil)
}

func NewFailIfNotTactic(p ProbeEvaluator) (Tactic, error) {
	return newConditionalTactic(conditionalFailIfNot, p, NewSkipTactic(), nil)
}

func NewIfNoProofsTactic(child Tactic) (Tactic, error) {
	return newConditionalTactic(conditionalIfNoProofs, nil, child, nil)
}

func NewIfNoUnsatCoresTactic(child Tactic) (Tactic, error) {
	return newConditionalTactic(conditionalIfNoCores, nil, child, nil)
}

func NewIfNoModelsTactic(child Tactic) (Tactic, error) {
	return newConditionalTactic(conditionalIfNoModels, nil, child, nil)
}

func (t *conditionalTactic) open() error {
	if t == nil || t.self != t || t.closed {
		return arena.ErrClosed
	}
	return nil
}

func (t *conditionalTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.references++
	return nil
}

func (t *conditionalTactic) DecRef() error {
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
	var err error
	if t.probe != nil {
		err = errors.Join(err, t.probe.DecRef())
		t.probe = nil
	}
	err = errors.Join(err, t.thenTactic.DecRef())
	t.thenTactic = nil
	if t.elseTactic != nil {
		err = errors.Join(err, t.elseTactic.DecRef())
		t.elseTactic = nil
	}
	return err
}

func (t *conditionalTactic) Apply(g *Goal, out *TacticResult) error {
	if err := t.open(); err != nil {
		return err
	}
	if g == nil || out == nil {
		return ast.ErrASTHandle
	}
	switch t.kind {
	case conditionalChoose, conditionalFailIf, conditionalFailIfNot:
		value, err := t.probe.Eval(g)
		if err != nil {
			return err
		}
		truth := ProbeIsTrue(value)
		if t.kind == conditionalFailIf && truth || t.kind == conditionalFailIfNot && !truth {
			return NewTacticException("fail-if tactic")
		}
		if t.kind != conditionalChoose {
			return t.thenTactic.Apply(g, out)
		}
		if truth {
			return t.thenTactic.Apply(g, out)
		}
		return t.elseTactic.Apply(g, out)
	case conditionalIfNoProofs:
		enabled, err := g.ProofsEnabled()
		return t.applyUnlessEnabled(g, out, enabled, err)
	case conditionalIfNoCores:
		enabled, err := g.UnsatCoresEnabled()
		return t.applyUnlessEnabled(g, out, enabled, err)
	case conditionalIfNoModels:
		enabled, err := g.ModelsEnabled()
		return t.applyUnlessEnabled(g, out, enabled, err)
	}
	return ast.ErrASTDomain
}

func (t *conditionalTactic) applyUnlessEnabled(g *Goal, out *TacticResult, enabled bool, err error) error {
	if err != nil {
		return err
	}
	if enabled {
		if err = out.prepare(g.manager); err != nil {
			return err
		}
		return out.AddGoal(g)
	}
	return t.thenTactic.Apply(g, out)
}

func (t *conditionalTactic) children() []Tactic {
	if t.elseTactic == nil {
		return []Tactic{t.thenTactic}
	}
	return []Tactic{t.thenTactic, t.elseTactic}
}

func (t *conditionalTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	var err error
	for _, child := range t.children() {
		err = errors.Join(err, child.UpdateParams(p))
	}
	return err
}
func (t *conditionalTactic) CollectParamDescrs(p *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	var err error
	for _, child := range t.children() {
		err = errors.Join(err, child.CollectParamDescrs(p))
	}
	return err
}
func (t *conditionalTactic) CollectStatistics() map[string]float64 {
	result := make(map[string]float64)
	if t.open() != nil {
		return result
	}
	for _, child := range t.children() {
		for name, value := range child.CollectStatistics() {
			result[name] += value
		}
	}
	return result
}
func (t *conditionalTactic) each(action func(Tactic) error) error {
	if err := t.open(); err != nil {
		return err
	}
	var err error
	for _, child := range t.children() {
		err = errors.Join(err, action(child))
	}
	return err
}
func (t *conditionalTactic) ResetStatistics() error {
	return t.each(func(c Tactic) error { return c.ResetStatistics() })
}
func (t *conditionalTactic) Cleanup() error {
	return t.each(func(c Tactic) error { return c.Cleanup() })
}
func (t *conditionalTactic) Reset() error { return t.each(func(c Tactic) error { return c.Reset() }) }
func (t *conditionalTactic) SetLogic(l usymbol.Symbol) error {
	return t.each(func(c Tactic) error { return c.SetLogic(l) })
}
func (t *conditionalTactic) SetProgressCallback(callback func(uint32)) error {
	return t.each(func(c Tactic) error { return c.SetProgressCallback(callback) })
}
func (t *conditionalTactic) Translate(m *ast.ASTManager) (Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if t.kind == conditionalFailIf || t.kind == conditionalFailIfNot {
		return t, nil
	}
	first, err := t.thenTactic.Translate(m)
	if err != nil {
		return nil, err
	}
	var second Tactic
	if t.elseTactic != nil {
		second, err = t.elseTactic.Translate(m)
		if err != nil {
			return nil, err
		}
	}
	return newConditionalTactic(t.kind, t.probe, first, second)
}
