package qe

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func checkQETargetServices(m *ast.ASTManager, config Config) error {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return uparams.ErrParamsOwner
	}
	return config.Context.CheckServices(m.Symbols, m.NumeralService())
}

// TranslateWithServices follows qe-sat's source translation, which constructs
// the target tactic with default parameters.
func (t *QESATTactic) TranslateWithServices(m *ast.ASTManager, config Config) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if err := checkQETargetServices(m, config); err != nil {
		return nil, err
	}
	next, err := NewQESATTactic(m, nil, config)
	if err != nil {
		return nil, err
	}
	return next, nil
}

// TranslateWithServices creates an independent QSAT search in the same mode.
func (t *QSATTactic) TranslateWithServices(m *ast.ASTManager, config Config) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if err := checkQETargetServices(m, config); err != nil {
		return nil, err
	}
	next, err := newQSATTactic(m, t.mode, config)
	if err != nil {
		return nil, err
	}
	return next, nil
}

// TranslateWithServices copies nonlinear QSAT parameters into the target.
func (t *NLQSATTactic) TranslateWithServices(m *ast.ASTManager, config Config) (result tactic.Tactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if err := checkQETargetServices(m, config); err != nil {
		return nil, err
	}
	p, err := t.config.Context.TranslateTo(&t.params, config.Context)
	if err != nil {
		return nil, err
	}
	var next *NLQSATTactic
	defer func() {
		resultErr = errors.Join(resultErr, config.Context.Del(&p))
		if resultErr != nil && next != nil {
			resultErr = errors.Join(resultErr, next.Close())
			result = nil
		}
	}()
	next, err = newNLQTactic(m, &p, config, t.mode)
	if err != nil {
		return nil, err
	}
	return next, nil
}
