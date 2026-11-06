package api

// Source: src/api/api_ast_map.cpp:27-55; src/api/api_ast_map.h:25-31.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// ErrASTMapMissing reports a lookup for a key that is not in an ASTMap.
var ErrASTMapMissing = errors.New("api: AST map key is absent")

// ASTMap is the owning Go surface for the source Z3_ast_map object. Keys and
// values may be expressions, declarations, or sorts. Close the map before its
// SMTContext, because the map borrows the context's AST manager.
type ASTMap struct {
	owner  *apiContextResults
	object *apiASTMapObject
}

// NewASTMap constructs an empty context-local AST map.
func (c *SMTContext) NewASTMap() (result *ASTMap, resultErr error) {
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
	object, err := newAPIASTMapObject(owner)
	if err != nil {
		return nil, err
	}
	if err = owner.saveObject(&object.object); err != nil {
		return nil, err
	}
	return &ASTMap{owner: owner, object: object}, nil
}

func (m *ASTMap) check() error {
	if m == nil || m.owner == nil || m.object == nil {
		return ast.ErrASTHandle
	}
	return m.object.check()
}

// Close releases every map-owned key and value reference.
func (m *ASTMap) Close() error {
	if m == nil {
		return ast.ErrASTHandle
	}
	if m.owner == nil {
		return nil
	}
	err := m.owner.Close()
	if err == nil {
		m.owner = nil
		m.object = nil
	}
	return err
}
