package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp:267-289 and
// src/tactic/bv/bv_bounds_tactic.cpp:407-432.

import "github.com/glycerine/g3/pkg/ast"

func (s *BVBoundsSimplifier) containsExpression(root, target ast.ASTHandle) (bool, error) {
	if s == nil || s.manager == nil || root.IsNull() || target.IsNull() {
		return false, ast.ErrASTDomain
	}
	todo := []ast.ASTHandle{root}
	marked := make(map[ast.ASTHandle]bool)
	for len(todo) != 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		if marked[current] {
			continue
		}
		if current == target {
			return true, nil
		}
		marked[current] = true
		kind, err := s.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		count, err := s.manager.GetNumArgs(current)
		if err != nil {
			return false, err
		}
		for i := uint32(0); i < count; i++ {
			argument, err := s.manager.GetArg(current, i)
			if err != nil {
				return false, err
			}
			todo = append(todo, argument)
		}
	}
	return false, nil
}
