package api

// Source: src/api/api_qe.cpp Z3_qe_model_project_skolem.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/qe"
)

// QEModelProjectSkolem projects bounds and publishes the final arithmetic
// substitution map. Mapping owns its inserted roots; result owns its formula.
func (c *SMTContext) QEModelProjectSkolem(source *model.ModelCore, bounds []ast.ASTHandle, body ast.ASTHandle, mapping *ASTMap) (result ast.ASTRef, resultErr error) {
	if err := c.checkQEModel(source); err != nil {
		return ast.ASTRef{}, err
	}
	manager, err := c.Manager()
	if err != nil {
		return ast.ASTRef{}, err
	}
	if body.IsNull() || mapping == nil || mapping.owner == nil || mapping.owner.manager != manager {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	if err = mapping.check(); err != nil {
		return ast.ASTRef{}, err
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
	expressions, err := ast.NewASTExprMapWithProofs(manager, false)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, expressions.Close()) }()
	projected, err := projector.ProjectSkolem(source, bounds, body, expressions)
	if err != nil {
		return ast.ASTRef{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, projected.Close()) }()
	if err = mapping.object.copyExprPairs(expressions); err != nil {
		return ast.ASTRef{}, err
	}
	formula, err := projected.Formula()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return manager.NewRef(formula)
}
