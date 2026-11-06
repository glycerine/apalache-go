package sls

// Source: src/tactic/sls/sls_tactic.h ADD_TACTIC factory declaration.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type QFBVFactory struct{ config smtcore.SMTSolverConfig }

func NewQFBVSLSFactory(config smtcore.SMTSolverConfig) tactic.TacticFactory {
	return &QFBVFactory{config: config}
}

func (f *QFBVFactory) New(manager *ast.ASTManager, params *uparams.ParamsRef) (tactic.Tactic, error) {
	if f == nil || manager == nil {
		return nil, ast.ErrASTHandle
	}
	config, err := QFBVConfigFromSolver(f.config)
	if err != nil {
		return nil, err
	}
	return NewQFBVSLSTactic(manager, params, config)
}

func (*QFBVFactory) Close() error { return nil }
