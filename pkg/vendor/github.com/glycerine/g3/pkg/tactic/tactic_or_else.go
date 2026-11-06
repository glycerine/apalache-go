package tactic

// Source: src/tactic/tactical.cpp:272-459,1385-1387.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type orElseTactic struct {
	self       *orElseTactic
	children   []Tactic
	references uint32
	closed     bool
}

func NewOrElseTactic(children ...Tactic) (Tactic, error) {
	if len(children) == 0 {
		return nil, ast.ErrASTDomain
	}
	t := &orElseTactic{children: make([]Tactic, 0, len(children))}
	t.self = t
	for _, child := range children {
		if child == nil {
			_ = t.releaseChildren()
			return nil, ast.ErrASTHandle
		}
		if err := child.IncRef(); err != nil {
			_ = t.releaseChildren()
			return nil, err
		}
		t.children = append(t.children, child)
	}
	return t, nil
}

func NewSkipIfFailedTactic(child Tactic) (Tactic, error) {
	return NewOrElseTactic(child, NewSkipTactic())
}

func (t *orElseTactic) open() error {
	if t == nil || t.self != t || t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *orElseTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.references++
	return nil
}
func (t *orElseTactic) DecRef() error {
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
	return t.releaseChildren()
}
func (t *orElseTactic) releaseChildren() error {
	var err error
	for i, child := range t.children {
		if child != nil {
			err = errors.Join(err, child.DecRef())
			t.children[i] = nil
		}
	}
	t.children = nil
	return err
}

func (t *orElseTactic) Apply(g *Goal, out *TacticResult) (result error) {
	if err := t.open(); err != nil {
		return err
	}
	if g == nil || out == nil {
		return ast.ErrASTHandle
	}
	snapshot, err := NewGoalConfigCopy(g)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, snapshot.Close()) }()
	if err = g.CopyTo(snapshot); err != nil {
		return err
	}
	for i, child := range t.children {
		if err = out.Reset(); err != nil {
			return err
		}
		err = child.Apply(g, out)
		if err == nil || i == len(t.children)-1 {
			return err
		}
		var tacticError TacticException
		if !errors.As(err, &tacticError) {
			return err
		}
		if err = g.ResetAll(); err != nil {
			return err
		}
		if err = snapshot.CopyTo(g); err != nil {
			return err
		}
	}
	return ast.ErrASTDomain
}

func (t *orElseTactic) each(action func(Tactic) error) error {
	if err := t.open(); err != nil {
		return err
	}
	var err error
	for _, child := range t.children {
		err = errors.Join(err, action(child))
	}
	return err
}
func (t *orElseTactic) UpdateParams(p *uparams.ParamsRef) error {
	return t.each(func(c Tactic) error { return c.UpdateParams(p) })
}
func (t *orElseTactic) CollectParamDescrs(p *uparams.ParamDescrs) error {
	return t.each(func(c Tactic) error { return c.CollectParamDescrs(p) })
}
func (t *orElseTactic) CollectStatistics() map[string]float64 {
	result := make(map[string]float64)
	if t.open() != nil {
		return result
	}
	for _, child := range t.children {
		for name, value := range child.CollectStatistics() {
			result[name] += value
		}
	}
	return result
}
func (t *orElseTactic) ResetStatistics() error {
	return t.each(func(c Tactic) error { return c.ResetStatistics() })
}
func (t *orElseTactic) Cleanup() error { return t.each(func(c Tactic) error { return c.Cleanup() }) }
func (t *orElseTactic) Reset() error   { return t.each(func(c Tactic) error { return c.Reset() }) }
func (t *orElseTactic) SetLogic(l usymbol.Symbol) error {
	return t.each(func(c Tactic) error { return c.SetLogic(l) })
}
func (t *orElseTactic) SetProgressCallback(callback func(uint32)) error {
	return t.each(func(c Tactic) error { return c.SetProgressCallback(callback) })
}
func (t *orElseTactic) Translate(m *ast.ASTManager) (Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	translated := make([]Tactic, 0, len(t.children))
	for _, child := range t.children {
		value, err := child.Translate(m)
		if err != nil {
			return nil, err
		}
		translated = append(translated, value)
	}
	return NewOrElseTactic(translated...)
}
