package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp:291-326 and
// src/tactic/bv/bv_bounds_tactic.cpp:434-468.

import "github.com/glycerine/g3/pkg/ast"

func (s *BVBoundsSimplifier) containsBound(root ast.ASTHandle) (bool, error) {
	if s == nil || s.manager == nil || root.IsNull() {
		return false, ast.ErrASTDomain
	}
	todo := []ast.ASTHandle{root}
	markedExpressions := make(map[ast.ASTHandle]bool)
	markedVariables := make(map[ast.ASTHandle]bool)
	for len(todo) != 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		if markedExpressions[current] {
			continue
		}
		markedExpressions[current] = true
		kind, err := s.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		variable, _, found, err := s.recognize(current)
		if err != nil {
			return false, err
		}
		if found {
			if markedVariables[variable] {
				return true, nil
			}
			markedVariables[variable] = true
			if _, exists := s.bounds[variable]; exists {
				return true, nil
			}
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
