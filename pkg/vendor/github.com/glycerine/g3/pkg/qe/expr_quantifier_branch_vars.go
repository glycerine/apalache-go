package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::add_var/get_branch_id.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type exprQEBranchVariable struct {
	variable, branch ast.ASTRef
	contains         *ast.ContainsApp
}
type exprQEBranchVariables struct {
	manager *ast.ASTManager
	entries []exprQEBranchVariable
	index   map[ast.ASTHandle]int
	pending []ast.ASTRef
}

func newExprQEBranchVariables(m *ast.ASTManager) (*exprQEBranchVariables, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	return &exprQEBranchVariables{manager: m, index: make(map[ast.ASTHandle]int)}, nil
}
func (s *exprQEBranchVariables) add(variable ast.ASTHandle) (err error) {
	if s == nil || s.manager == nil {
		return ast.ErrASTHandle
	}
	m := s.manager
	ref, err := m.NewRef(variable)
	if err != nil {
		return err
	}
	s.pending = append(s.pending, ref)
	if _, ok := s.index[variable]; ok {
		return nil
	}
	contains, err := ast.NewContainsApp(m, variable)
	if err != nil {
		return err
	}
	owned, err := m.NewRef(variable)
	if err != nil {
		return errors.Join(err, contains.Close())
	}
	s.entries = append(s.entries, exprQEBranchVariable{variable: owned, contains: contains})
	entry := &s.entries[len(s.entries)-1]
	varSort, err := m.GetSort(variable)
	if err != nil {
		return err
	}
	boolSort, err := m.MkBoolSort()
	if err != nil {
		return err
	}
	isBV, err := m.IsBitVectorSort(varSort)
	if err != nil {
		return err
	}
	branch := variable
	if varSort != boolSort && !isBV {
		bvSort, e := m.BitVectorSort(20)
		if e != nil {
			return e
		}
		branch, e = m.MkFreshConst("b", bvSort)
		if e != nil {
			return e
		}
	}
	entry.branch, err = m.NewRef(branch)
	if err != nil {
		return err
	}
	s.index[variable] = len(s.entries) - 1
	return nil
}
func (s *exprQEBranchVariables) branch(variable ast.ASTHandle) (ast.ASTHandle, error) {
	if s == nil || s.manager == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	i, found := s.index[variable]
	if !found {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return s.manager.RefGet(s.entries[i].branch)
}
func (s *exprQEBranchVariables) consume(node *exprQETree) error {
	if s == nil || s.manager == nil || node == nil || node.manager != s.manager {
		return ast.ErrASTHandle
	}
	values := make([]ast.ASTHandle, len(s.pending))
	for i, ref := range s.pending {
		h, err := s.manager.RefGet(ref)
		if err != nil {
			return err
		}
		values[i] = h
	}
	if err := node.consumeVariables(values); err != nil {
		return err
	}
	for len(s.pending) > 0 {
		i := len(s.pending) - 1
		if err := s.manager.DelRef(&s.pending[i]); err != nil {
			return err
		}
		s.pending = s.pending[:i]
	}
	return nil
}
func (s *exprQEBranchVariables) Close() error {
	if s == nil || s.manager == nil {
		return nil
	}
	for len(s.pending) > 0 {
		i := len(s.pending) - 1
		if err := s.manager.DelRef(&s.pending[i]); err != nil {
			return err
		}
		s.pending = s.pending[:i]
	}
	for len(s.entries) > 0 {
		i := len(s.entries) - 1
		e := &s.entries[i]
		if e.contains != nil {
			if err := e.contains.Close(); err != nil {
				return err
			}
			e.contains = nil
		}
		for _, ref := range []*ast.ASTRef{&e.branch, &e.variable} {
			if *ref != (ast.ASTRef{}) {
				if err := s.manager.DelRef(ref); err != nil {
					return err
				}
			}
		}
		s.entries = s.entries[:i]
	}
	s.index = nil
	s.manager = nil
	return nil
}
