package api

// Source: src/api/api_opt.cpp objective handles, lower/upper state and
// get-assertions/get-objectives.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
)

type APIOptimizeObjective = cmd_context.OptimizationObjective
type APIOptimizeObjectiveValue = cmd_context.OptimizationObjectiveValue

func (o *APIOptimize) Assertions() ([]ast.ASTHandle, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	return o.command.Assertions()
}

func (o *APIOptimize) NumObjectives() (uint32, error) {
	if err := o.open(); err != nil {
		return 0, err
	}
	return o.command.ObjectiveCount()
}

func (o *APIOptimize) Objective(index uint32) (APIOptimizeObjective, error) {
	if err := o.open(); err != nil {
		return APIOptimizeObjective{}, err
	}
	return o.command.Objective(index)
}

func (o *APIOptimize) ObjectiveValue(index uint32) (APIOptimizeObjectiveValue, error) {
	if err := o.open(); err != nil {
		return APIOptimizeObjectiveValue{}, err
	}
	return o.command.ObjectiveValue(index)
}
