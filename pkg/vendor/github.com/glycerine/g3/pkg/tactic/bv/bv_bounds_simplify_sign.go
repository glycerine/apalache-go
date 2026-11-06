package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp:227-239,262-263.

import "github.com/glycerine/g3/pkg/ast"

func (s *BVBoundsSimplifier) stripBoundNots(h ast.ASTHandle) (ast.ASTHandle, bool, error) {
	sign := false
	for {
		not, argument, err := s.manager.IsNot(h)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if !not {
			return h, sign, nil
		}
		h = argument
		sign = !sign
	}
}

func (s *BVBoundsSimplifier) finishBoundSign(value ast.ASTHandle, sign bool) (ast.ASTHandle, error) {
	if !sign {
		return value, nil
	}
	return s.manager.MkNot(value)
}
