package portfolio

// Source: src/tactic/fd_solver/fd_solver.h mk_fd_tactic factory boundary.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type FDTacticFactory struct {
	services model.ModelEvaluatorConfig
	closed   bool
}

func NewFDTacticFactory(services model.ModelEvaluatorConfig) (*FDTacticFactory, error) {
	if services.Context == nil || services.Globals == nil || services.Globals.Context != services.Context {
		return nil, uparams.ErrParamsOwner
	}
	return &FDTacticFactory{services: services}, nil
}
func (f *FDTacticFactory) New(m *ast.ASTManager, p *uparams.ParamsRef) (tactic.Tactic, error) {
	if f == nil || f.closed {
		return nil, uparams.ErrParamsOwner
	}
	return NewFDTactic(m, p, f.services)
}
func (f *FDTacticFactory) Close() error {
	if f == nil || f.closed {
		return nil
	}
	f.closed = true
	f.services = model.ModelEvaluatorConfig{}
	return nil
}

var _ tactic.TacticFactory = (*FDTacticFactory)(nil)
