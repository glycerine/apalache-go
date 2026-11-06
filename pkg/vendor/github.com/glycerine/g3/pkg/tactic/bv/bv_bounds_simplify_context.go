package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp:212-264.

import "github.com/glycerine/g3/pkg/ast"

func (s *BVBoundsSimplifier) simplifyRecognizedBound(key ast.ASTHandle, want bvBoundsSet, sign bool) (ast.ASTHandle, bool, error) {
	if sign && want.tight {
		sign = false
		want = want.sourceComplement()
		if want.empty() {
			value, err := s.manager.MkFalse()
			return value, err == nil, err
		}
	}

	var value ast.ASTHandle
	var err error
	if want.full() && want.tight {
		value, err = s.manager.MkTrue()
	} else if have, exists := s.bounds[key]; exists {
		if have.implies(want) {
			value, err = s.manager.MkTrue()
		} else {
			intersection := have.intersect(want)
			if intersection.empty() {
				value, err = s.manager.MkFalse()
			} else if s.propagateEq {
				if singleton, yes := intersection.singleton(); yes {
					var numeral ast.ASTHandle
					numeral, err = bvBoundsMakeNumeral(s.manager, singleton, want.width)
					if err == nil {
						value, err = s.manager.MkEq(key, numeral)
					}
				}
			}
		}
	}
	if err != nil || value.IsNull() {
		return ast.ASTHandle{}, false, err
	}
	value, err = s.finishBoundSign(value, sign)
	return value, err == nil, err
}
