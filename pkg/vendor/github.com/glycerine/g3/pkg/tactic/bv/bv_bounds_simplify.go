package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp simplify_core/may_simplify.

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (s *BVBoundsSimplifier) MaySimplify(h ast.ASTHandle) (bool, error) {
	if s == nil || s.manager == nil || h.IsNull() {
		return false, ast.ErrASTDomain
	}
	if _, _, numeral, err := bvBoundsNumeral(s.manager, h); err != nil || numeral {
		return false, err
	}
	for {
		not, argument, err := s.manager.IsNot(h)
		if err != nil {
			return false, err
		}
		if !not {
			break
		}
		h = argument
	}
	for variable := range s.bounds {
		contains, err := s.containsExpression(h, variable)
		if err != nil || contains {
			return contains, err
		}
	}
	variable, bound, found, err := s.recognize(h)
	if err != nil {
		return false, err
	}
	if found {
		_, contextual := s.bounds[variable]
		return bound.full() || contextual, nil
	}
	return s.containsBound(h)
}
func (s *BVBoundsSimplifier) Simplify(h ast.ASTHandle, out ast.ASTRef) (bool, error) {
	if b, ok := s.bounds[h]; ok {
		if value, single := b.singleton(); single {
			v, e := bvBoundsMakeNumeral(s.manager, value, b.width)
			if e != nil {
				return false, e
			}
			return true, s.manager.SetRef(out, v)
		}
	}
	core, sign, e := s.stripBoundNots(h)
	if e != nil {
		return false, e
	}
	key, want, ok, e := s.recognize(core)
	if e != nil || !ok {
		return false, e
	}
	value, done, e := s.simplifyRecognizedBound(key, want, sign)
	if e != nil || !done {
		return false, e
	}
	return true, s.manager.SetRef(out, value)
}
