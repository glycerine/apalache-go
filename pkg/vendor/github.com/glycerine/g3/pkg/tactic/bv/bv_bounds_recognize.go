package bv

// Source: src/tactic/bv/bv_bounds_tactic.cpp is_bound.

import (
	"github.com/glycerine/g3/pkg/ast"
	"math/big"
)

func (s *BVBoundsSimplifier) recognize(h ast.ASTHandle) (variable ast.ASTHandle, bound bvBoundsSet, found bool, err error) {
	m := s.manager
	var lhs, rhs ast.ASTHandle
	unsigned, e := m.BitVectorULEOperands(h, &lhs, &rhs)
	if e != nil {
		return variable, bound, false, e
	}
	signed := false
	if !unsigned {
		signed, e = m.BitVectorSLEOperands(h, &lhs, &rhs)
		if e != nil {
			return variable, bound, false, e
		}
	}
	if unsigned || signed {
		lv, w, ln, e := bvBoundsNumeral(m, lhs)
		if e != nil {
			return variable, bound, false, e
		}
		rv, rw, rn, e := bvBoundsNumeral(m, rhs)
		if e != nil {
			return variable, bound, false, e
		}
		// bv_bounds_tactic.cpp::is_number rejects every numeral wider than
		// uint64, even though the shared modular interval type supports them.
		if ln && w > 64 || rn && rw > 64 {
			return variable, bound, false, nil
		}
		if ln && rn {
			return variable, bound, false, nil
		}
		if !ln {
			w = rw
		}
		if ln {
			variable = rhs
			if signed {
				bound = bvBoundsSignedLower(w, lv)
			} else {
				max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(w)), big.NewInt(1))
				bound = bvBoundsRangeSet(w, lv, max)
			}
		} else if rn {
			variable = lhs
			if signed {
				bound = bvBoundsSignedUpper(w, rv)
			} else {
				bound = bvBoundsRangeSet(w, big.NewInt(0), rv)
			}
		} else {
			return variable, bound, false, nil
		}
		return variable, bound, true, nil
	}
	if k, e := m.GetKind(h); e != nil || k != ast.ASTApp {
		return variable, bound, false, e
	}
	d, e := m.GetDecl(h)
	if e != nil {
		return variable, bound, false, e
	}
	fam, e := m.GetFamilyID(d)
	if e != nil {
		return variable, bound, false, e
	}
	op, e := m.GetDeclKind(d)
	if e != nil || fam != m.BasicFamilyID() || op != ast.ASTOpEq {
		return variable, bound, false, e
	}
	lhs, e = m.GetArg(h, 0)
	if e != nil {
		return variable, bound, false, e
	}
	rhs, e = m.GetArg(h, 1)
	if e != nil {
		return variable, bound, false, e
	}
	v, w, yes, e := bvBoundsNumeral(m, lhs)
	if e != nil {
		return variable, bound, false, e
	}
	if yes && w > 64 {
		yes = false
	}
	if !yes {
		v, w, yes, e = bvBoundsNumeral(m, rhs)
		lhs, rhs = rhs, lhs
	}
	if e != nil || !yes {
		return variable, bound, false, e
	}
	if w > 64 {
		return variable, bound, false, nil
	}
	return rhs, bvBoundsSingleton(w, v), true, nil
}
func bvBoundsSignedLower(w uint32, v *big.Int) bvBoundsSet {
	sign := new(big.Int).Lsh(big.NewInt(1), uint(w-1))
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(w)), big.NewInt(1))
	if v.Cmp(sign) >= 0 {
		return bvBoundsSet{width: w, ranges: []bvBoundsRange{{bvBI(v), max}, {big.NewInt(0), new(big.Int).Sub(sign, big.NewInt(1))}}, tight: true}.normalize()
	}
	return bvBoundsRangeSet(w, v, new(big.Int).Sub(sign, big.NewInt(1)))
}
func bvBoundsSignedUpper(w uint32, v *big.Int) bvBoundsSet {
	sign := new(big.Int).Lsh(big.NewInt(1), uint(w-1))
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(w)), big.NewInt(1))
	if v.Cmp(sign) >= 0 {
		return bvBoundsRangeSet(w, sign, v)
	}
	return bvBoundsSet{width: w, ranges: []bvBoundsRange{{sign, max}, {big.NewInt(0), bvBI(v)}}, tight: true}.normalize()
}
