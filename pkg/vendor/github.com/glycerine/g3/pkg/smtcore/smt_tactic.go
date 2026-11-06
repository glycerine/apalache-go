package smtcore

// Source: src/smt/tactic/smt_tactic.h:30-32;
// src/smt/tactic/smt_tactic.cpp:289-300.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// NewSMTTacticUsing constructs the retained SMT solver tactic and fixes its
// auto_config parameter with the same precedence as native using_params.
func NewSMTTacticUsing(m *ast.ASTManager, p *uparams.ParamsRef, logic string, autoConfig bool, config SMTSolverConfig) (result tactic.Tactic, err error) {
	if m == nil || config.Params == nil {
		return nil, ast.ErrASTHandle
	}
	params, err := config.Params.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, config.Params.Del(&params)) }()
	if p != nil {
		if err = config.Params.Assign(&params, p); err != nil {
			return nil, err
		}
	}
	autoName := "auto_config"
	if err = config.Params.SetBoolName(&params, &autoName, autoConfig); err != nil {
		return nil, err
	}
	base, err := NewSMTTactic(m, &params, config)
	if err != nil {
		return nil, err
	}
	if logic != "" {
		sym, e := m.Symbols.Intern(logic)
		if e == nil {
			e = base.SetLogic(sym)
		}
		if e != nil {
			return nil, errors.Join(e, base.Close())
		}
	}
	if err = base.traceUsingFactory(autoConfig, &params); err != nil {
		return nil, errors.Join(err, base.Close())
	}
	result, err = tactic.NewUsingParamsTactic(config.Params, base, &params)
	if err != nil {
		return nil, errors.Join(err, base.Close())
	}
	return result, nil
}
