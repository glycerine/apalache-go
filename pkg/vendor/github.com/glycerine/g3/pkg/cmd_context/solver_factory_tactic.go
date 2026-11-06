package cmd_context

// Source: src/api/api_solver.cpp:106-115.

import (
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
)

func (c *Context) NewSolverFromTactic(value tactic.Tactic) (solver.Solver, error) {
	if err := c.valid(); err != nil {
		return nil, err
	}
	factory, err := solver.NewTacticSolverFactory(value, solver.TacticSolverConfig{
		Params: c.config.Params,
		EvaluatorFactory: func(md *model.ModelCore) (solver.ConsequenceEvaluator, error) {
			return model.NewModelEvaluator(md, c.config.ModelEvaluator)
		},
		Trace: c.config.ModelEvaluator.Trace,
	})
	if err != nil {
		return nil, err
	}
	return c.newSolverFromFactory(factory, c.logic)
}
