package api

// Source: src/api/api_opt.cpp:70-136.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (o *APIOptimize) Assert(formula ast.ASTHandle) error {
	if err := o.open(); err != nil {
		return err
	}
	return o.command.Assert(formula, "")
}

func (o *APIOptimize) AssertSoft(formula ast.ASTHandle, weight *big.Rat, id string) (uint32, error) {
	if err := o.open(); err != nil {
		return 0, err
	}
	return o.command.AddSoftConstraintIndexed(formula, weight, id)
}

func (o *APIOptimize) Maximize(expression ast.ASTHandle) (uint32, error) {
	return o.addObjective(expression, true)
}

func (o *APIOptimize) Minimize(expression ast.ASTHandle) (uint32, error) {
	return o.addObjective(expression, false)
}

func (o *APIOptimize) addObjective(expression ast.ASTHandle, maximize bool) (uint32, error) {
	if err := o.open(); err != nil {
		return 0, err
	}
	index, err := o.command.ObjectiveCount()
	if err != nil {
		return 0, err
	}
	if err = o.command.AddObjective(expression, maximize); err != nil {
		return 0, err
	}
	return index, nil
}

func (o *APIOptimize) Push() error {
	if err := o.open(); err != nil {
		return err
	}
	return o.command.Push(1)
}

func (o *APIOptimize) Pop() error {
	if err := o.open(); err != nil {
		return err
	}
	return o.command.Pop(1)
}
