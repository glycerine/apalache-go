package api

// Source: src/api/api_qe.cpp Z3_model_extrapolate.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

// ModelExtrapolate returns a model-satisfying implicant of formula. The
// returned reference owns the conjunction.
func (c *SMTContext) ModelExtrapolate(source *model.ModelCore, formula ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if err := c.checkQEModel(source); err != nil {
		return ast.ASTRef{}, err
	}
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	facts, err := manager.FlattenAnd([]ast.ASTHandle{formula})
	if err != nil {
		return ast.ASTRef{}, err
	}
	services, err := c.command.ModelEvaluatorConfig()
	if err != nil {
		return ast.ASTRef{}, err
	}
	implicant, err := model.NewModelImplicant(manager, services)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, implicant.Close()) }()
	literals, err := implicant.MinimizeLiterals(facts, source.DerivedModel())
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, model.CloseImplicantResults(manager, &literals)) }()
	handles := make([]ast.ASTHandle, len(literals))
	for index := range literals {
		handles[index], err = manager.RefGet(literals[index])
		if err != nil {
			return ast.ASTRef{}, err
		}
	}
	conjunction, err := manager.MkAndArray(handles)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return manager.NewRef(conjunction)
}
