package api

// Source: src/api/api_qe.cpp Z3_qe_model_project.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/qe"
)

// QEModelProject projects bounds from body under model. The returned
// reference owns the projected formula.
func (c *SMTContext) QEModelProject(source *model.ModelCore, bounds []ast.ASTHandle, body ast.ASTHandle) (result ast.ASTRef, resultErr error) {
	if err := c.checkQEModel(source); err != nil {
		return ast.ASTRef{}, err
	}
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if body.IsNull() {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	if err = apiQEApplications(manager, bounds); err != nil {
		return ast.ASTRef{}, err
	}
	config, err := c.apiQEConfig()
	if err != nil {
		return ast.ASTRef{}, err
	}
	projector, err := qe.NewLegacyProjection(manager, config)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, projector.Close()) }()
	projected, err := projector.Project(source, bounds, body, false, false)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, projected.Close()) }()
	formula, err := projected.Formula()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return manager.NewRef(formula)
}
