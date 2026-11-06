package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp simplifier::translate.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic/core"
)

func (s *BVBoundsSimplifier) Translate(m *ast.ASTManager) (core.ContextSimplifier, error) {
	n := NewBVBoundsSimplifier(m, s.context)
	n.propagateEq = s.propagateEq
	return n, nil
}

// Rewrite adapts the bit-vector bounds callback to dom_simplify_tactic's
// expression-reference contract.
func (s *BVBoundsSimplifier) Rewrite(out ast.ASTRef) error {
	h, err := s.manager.RefGet(out)
	if err != nil {
		return err
	}
	_, err = s.Simplify(h, out)
	return err
}

func (s *BVBoundsSimplifier) TranslateDom(m *ast.ASTManager) (core.DomSimplifier, error) {
	n := NewBVBoundsSimplifier(m, s.context)
	n.propagateEq = s.propagateEq
	return n, nil
}
