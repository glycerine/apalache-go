package portfolio

// Source: src/tactic/fd_solver/fd_solver.cpp mk_seq_fd_tactic and mk_fd_tactic.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// NewFDTactic selects the source sequential finite-domain stack used by the
// current single-goroutine build profile.
func NewFDTactic(m *ast.ASTManager, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	return NewFDTacticWithOutput(m, p, services, nil)
}

func NewFDTacticWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, services model.ModelEvaluatorConfig, output io.Writer) (result tactic.Tactic, err error) {
	if m == nil || services.Context == nil {
		return nil, uparams.ErrParamsOwner
	}
	backend, err := NewFDSolverWithOutput(m, p, services, output)
	if err != nil {
		return nil, err
	}
	converted, err := solver.NewSolverTactic(backend, solver.SolverTacticConfig{Params: services.Context})
	if err != nil {
		return nil, errors.Join(err, backend.Close())
	}
	return converted, nil
}
