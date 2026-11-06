package rewriter

// Source: src/ast/rewriter/var_subst.cpp:26-40;
// src/ast/rewriter/var_subst.h:35-53.

import (
	"github.com/glycerine/g3/pkg/ast"
	"reflect"
)

// VarSubstitution implements the original var_subst functor with an embedded
// beta reducer. Standard order binds variable zero to the last argument; false
// binds variable zero to the first argument, as func_interp::eval_else requires.
// Each invocation resets the original reducer before installing new bindings.
type VarSubstitution struct {
	reducer       betaReducer
	standardOrder bool
}

// NewVarSubstitution charges the complete enclosing owner once. If member
// construction fails, the returned partial owner must still be closed.
func NewVarSubstitution(m *ast.ASTManager, standardOrder bool) (*VarSubstitution, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(VarSubstitution{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	s := &VarSubstitution{standardOrder: standardOrder}
	s.reducer.cfg.manager = m
	s.reducer.engine.header = bytes
	return s, s.reducer.engine.init(m, &s.reducer.cfg)
}
func (s *VarSubstitution) StandardOrder() bool { return s.standardOrder }
func (s *VarSubstitution) Substitute(term ast.ASTHandle, args []ast.ASTHandle, out ast.ASTRef) error {
	if uint64(len(args)) > uint64(^uint32(0)) {
		return ast.ErrASTDomain
	}
	return s.SubstituteArguments(term, ast.SliceArguments(args), out)
}

// SubstituteArguments preserves the original binding copies while borrowing the
// input argument view, without an additional pointer-array allocation.
func (s *VarSubstitution) SubstituteArguments(term ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) error {
	if s == nil {
		return ast.ErrASTHandle
	}
	r := &s.reducer.engine
	if e := r.open(); e != nil {
		return e
	}
	if e := s.assertWellSorted(term, 27); e != nil {
		return e
	}
	if e := r.Reset(); e != nil {
		return e
	}
	if e := r.setBindingArguments(args, s.standardOrder); e != nil {
		return e
	}
	if e := r.Rewrite(term, out); e != nil {
		return e
	}
	if e := s.assertResultWellSorted(out); e != nil {
		return e
	}
	return s.traceSubstitution(term, args, out)
}
func (s *VarSubstitution) Reset() error {
	if s == nil {
		return ast.ErrASTHandle
	}
	return s.reducer.engine.Reset()
}
func (s *VarSubstitution) Close() error {
	if s == nil {
		return ast.ErrASTHandle
	}
	return s.reducer.Close()
}
