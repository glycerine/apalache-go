package arith

// Source: src/tactic/arith/eq2bv_tactic.cpp, generated finite-domain bounds.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type eq2bvBound struct {
	formula    ast.ASTRef
	dependency ast.ASTDependency
}
type eq2bvBounds struct {
	manager *ast.ASTManager
	entries []eq2bvBound
}

func (b *eq2bvBounds) Close() error {
	if b == nil || b.manager == nil {
		return nil
	}
	for len(b.entries) != 0 {
		i := len(b.entries) - 1
		entry := &b.entries[i]
		if entry.formula != (ast.ASTRef{}) {
			if err := b.manager.DelRef(&entry.formula); err != nil {
				return err
			}
		}
		if !entry.dependency.IsNull() {
			consumed, err := b.manager.DependencyDecConsumed(entry.dependency)
			if consumed {
				entry.dependency = ast.ASTDependency{}
			}
			if err != nil {
				return err
			}
			if !consumed {
				return ast.ErrASTPending
			}
		}
		b.entries = b.entries[:i]
	}
	b.manager = nil
	return nil
}
func eq2bvNumeral(m *ast.ASTManager, value *big.Rat, width uint32) (h ast.ASTHandle, err error) {
	service := m.NumeralService()
	n, err := service.NewValue()
	if err != nil {
		return h, err
	}
	defer func() { err = errors.Join(err, service.Del(&n)) }()
	if err = service.SetString(&n, value.RatString()); err != nil {
		return h, err
	}
	return m.BitVectorNumeral(&n, width)
}
func eq2bvMakeBounds(plan *eq2bvReplacements, source *BoundManager) (result *eq2bvBounds, err error) {
	if plan == nil || plan.manager == nil || source == nil || source.Manager() != plan.manager {
		return nil, ast.ErrASTHandle
	}
	m := plan.manager
	result = &eq2bvBounds{manager: m}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	for _, entry := range plan.entries {
		fresh, e := m.RefGet(entry.fresh)
		if e != nil {
			return result, e
		}
		for _, lower := range []bool{true, false} {
			var value big.Rat
			var strict, found bool
			var dep ast.ASTDependency
			if lower {
				value, strict, found, e = source.Lower(entry.original)
			} else {
				value, strict, found, e = source.Upper(entry.original)
			}
			if e != nil {
				return result, e
			}
			if !found || value.Sign() < 0 {
				continue
			}
			if strict {
				return result, ast.ErrASTDomain
			}
			if lower {
				dep, e = source.LowerDependency(entry.original)
			} else {
				dep, e = source.UpperDependency(entry.original)
			}
			if e != nil {
				return result, e
			}
			n, e := eq2bvNumeral(m, &value, entry.width)
			if e != nil {
				return result, e
			}
			left, right := fresh, n
			if lower {
				left, right = n, fresh
			}
			formula, e := m.MkBitVectorULE(left, right)
			if e != nil {
				return result, e
			}
			if !lower && entry.negativeSentinel {
				boundRef, x := m.NewRef(formula)
				if x != nil {
					return result, x
				}
				max := new(big.Int).Lsh(big.NewInt(1), uint(entry.width))
				max.Sub(max, big.NewInt(1))
				sentinel, x := eq2bvNumeral(m, new(big.Rat).SetInt(max), entry.width)
				if x == nil {
					sentinel, x = m.MkEq(fresh, sentinel)
				}
				if x == nil {
					var sentinelRef ast.ASTRef
					sentinelRef, x = m.NewRef(sentinel)
					if x == nil {
						formula, x = m.MkOr(formula, sentinel)
						x = errors.Join(x, m.DelRef(&sentinelRef))
					}
				}
				x = errors.Join(x, m.DelRef(&boundRef))
				if x != nil {
					return result, x
				}
			}
			ref, e := m.NewRef(formula)
			if e != nil {
				return result, e
			}
			if !dep.IsNull() {
				if e = m.DependencyInc(dep); e != nil {
					return result, errors.Join(e, m.DelRef(&ref))
				}
			}
			result.entries = append(result.entries, eq2bvBound{formula: ref, dependency: dep})
		}
	}
	return result, nil
}
