package api

// Source: src/api/api_ast_vector.cpp public owning surface.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// ASTVector owns one AST reference per non-null element and borrows the
// creating SMTContext's manager. Close it before closing that context.
type ASTVector struct {
	owner  *apiContextResults
	object *apiASTVectorObject
}

func (c *SMTContext) NewASTVector() (result *ASTVector, resultErr error) {
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	owner, err := newAPIContextResults(manager, true)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, owner.Close())
			result = nil
		}
	}()
	object, err := newAPIASTVectorObject(owner)
	if err != nil {
		return nil, err
	}
	if err = owner.saveObject(&object.object); err != nil {
		return nil, err
	}
	return &ASTVector{owner: owner, object: object}, nil
}

func (v *ASTVector) check() error {
	if v == nil || v.owner == nil || v.object == nil {
		return ast.ErrASTHandle
	}
	return v.object.check()
}

func (v *ASTVector) Close() error {
	if v == nil {
		return ast.ErrASTHandle
	}
	if v.owner == nil {
		return nil
	}
	err := v.owner.Close()
	if err == nil {
		v.owner, v.object = nil, nil
	}
	return err
}
