package portfolio

// Source: src/tactic/portfolio/default_tactic.cpp lazy mk_fd_tactic branch.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func NewLazyFDTactic(m *ast.ASTManager, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	return tactic.NewLazyTactic(m, services.Context, p, func(target *ast.ASTManager, settings *uparams.ParamsRef) (tactic.Tactic, error) {
		return NewFDTactic(target, settings, services)
	})
}
