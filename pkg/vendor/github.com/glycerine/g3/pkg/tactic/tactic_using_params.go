package tactic

// Source: src/tactic/tactical.cpp:1158-1188.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type usingParamsTactic struct {
	*unaryTactic
	context *uparams.ParamsContext
	params  uparams.ParamsRef
}

func NewUsingParamsTactic(context *uparams.ParamsContext, child Tactic, p *uparams.ParamsRef) (Tactic, error) {
	if context == nil || p == nil {
		return nil, ast.ErrASTHandle
	}
	if err := context.Check(p); err != nil {
		return nil, err
	}
	stored, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	if err = context.Assign(&stored, p); err != nil {
		return nil, errors.Join(err, context.Del(&stored))
	}
	base, err := newUnaryTactic(unaryUsingParams, child, 0)
	if err != nil {
		return nil, errors.Join(err, context.Del(&stored))
	}
	t := &usingParamsTactic{unaryTactic: base, context: context, params: stored}
	if err = child.UpdateParams(&t.params); err != nil {
		base.closed = true
		base.child = nil
		return nil, errors.Join(err, child.DecRef(), context.Del(&t.params))
	}
	return t, nil
}

func (t *usingParamsTactic) IncRef() error {
	return t.unaryTactic.IncRef()
}

func (t *usingParamsTactic) DecRef() error {
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
	child := t.child
	t.child = nil
	return errors.Join(child.DecRef(), t.context.Del(&t.params))
}

func (t *usingParamsTactic) UpdateParams(p *uparams.ParamsRef) (result error) {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		return ast.ErrASTHandle
	}
	combined, err := t.context.NewValue()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, t.context.Del(&combined)) }()
	if err = t.context.Assign(&combined, p); err != nil {
		return err
	}
	if err = t.context.Append(&combined, &t.params); err != nil {
		return err
	}
	return t.child.UpdateParams(&combined)
}

func (t *usingParamsTactic) Translate(m *ast.ASTManager) (Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	child, err := t.child.Translate(m)
	if err != nil {
		return nil, err
	}
	if err = child.IncRef(); err != nil {
		return nil, err
	}
	translated, err := NewUsingParamsTactic(t.context, child, &t.params)
	releaseErr := child.DecRef()
	if err != nil || releaseErr != nil {
		return nil, errors.Join(err, releaseErr)
	}
	return translated, nil
}
