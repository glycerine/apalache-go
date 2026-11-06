package api

// Source: src/api/api_polynomial.cpp:48-79 Z3_ast_vector result ownership.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// APIPolynomialSubresultants owns the AST references produced by one
// subresultant computation. Close it before closing its SMTContext.
type APIPolynomialSubresultants struct {
	owner   *smtContextState
	manager *ast.ASTManager
	values  []ast.ASTRef
	cursor  int
	closed  bool
}

func (r *APIPolynomialSubresultants) open() error {
	if r == nil || r.owner == nil || r.manager == nil || r.closed || r.owner.closed {
		return ErrSMTContextOwner
	}
	return nil
}

func (r *APIPolynomialSubresultants) Len() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return uint32(len(r.values)), nil
}

func (r *APIPolynomialSubresultants) Get(index uint32) (ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if uint64(index) >= uint64(len(r.values)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return r.manager.RefGet(r.values[index])
}

func (r *APIPolynomialSubresultants) Values() ([]ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, len(r.values))
	for i := range r.values {
		value, err := r.manager.RefGet(r.values[i])
		if err != nil {
			return nil, err
		}
		result[i] = value
	}
	return result, nil
}

func (r *APIPolynomialSubresultants) Close() error {
	if r == nil || r.closed {
		return nil
	}
	if r.manager == nil {
		return ErrSMTContextOwner
	}
	for r.cursor < len(r.values) {
		index := len(r.values) - r.cursor - 1
		err := r.manager.DelRef(&r.values[index])
		if r.values[index] == (ast.ASTRef{}) {
			r.cursor++
		}
		if err != nil {
			return err
		}
	}
	r.values = nil
	r.owner, r.manager = nil, nil
	r.closed = true
	return nil
}
