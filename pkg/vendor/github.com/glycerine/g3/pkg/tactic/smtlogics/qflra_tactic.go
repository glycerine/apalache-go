package smtlogics

// Source: src/tactic/smtlogics/qflra_tactic.cpp/.h, live #else factory.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// NewQFLRATactic preserves both source using-params wrappers: the fixed pivot
// setting is innermost, while the caller's initial params remain outermost.
func NewQFLRATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (result tactic.Tactic, err error) {
	c, g := config.Params, config.Globals
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	defer func() {
		if err != nil && result != nil {
			if e := result.IncRef(); e != nil {
				err = errors.Join(err, e)
			} else {
				err = errors.Join(err, result.DecRef())
			}
			result = nil
		}
	}()
	pivot, err := c.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, c.Del(&pivot)) }()
	name := "arith.greatest_error_pivot"
	if err = c.SetBoolName(&pivot, &name, true); err != nil {
		return nil, err
	}
	backend, err := smtcore.NewSMTTactic(m, nil, config)
	if err != nil {
		return nil, err
	}
	if err = backend.IncRef(); err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, backend.DecRef()) }()
	inner, err := tactic.NewUsingParamsTactic(c, backend, &pivot)
	if err != nil {
		return nil, err
	}
	if err = inner.IncRef(); err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, inner.DecRef()) }()
	if p == nil {
		empty := c.EmptyValue()
		p = &empty
	}
	return tactic.NewUsingParamsTactic(c, inner, p)
}
