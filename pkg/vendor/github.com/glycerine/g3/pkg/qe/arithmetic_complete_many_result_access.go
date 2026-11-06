package qe

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

func (r *ArithmeticCompleteProjectionResult) open() error {
	if r == nil || r.self != r || r.manager == nil {
		return ast.ErrASTHandle
	}
	if r.closed {
		return arena.ErrClosed
	}
	return nil
}
func (r *ArithmeticCompleteProjectionResult) Len() (uint32, error) {
	if e := r.open(); e != nil {
		return 0, e
	}
	return uint32(len(r.variables)), nil
}
func (r *ArithmeticCompleteProjectionResult) Variable(i uint32) (ast.ASTHandle, error) {
	if e := r.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	if i >= uint32(len(r.variables)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return r.manager.RefGet(r.variables[i])
}
func (r *ArithmeticCompleteProjectionResult) Definition(i uint32) (ast.ASTHandle, error) {
	if e := r.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	if i >= uint32(len(r.definitions)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return r.manager.RefGet(r.definitions[i])
}
