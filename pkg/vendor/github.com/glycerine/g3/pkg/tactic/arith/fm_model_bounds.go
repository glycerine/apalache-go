package arith

// Source: src/tactic/arith/fm_tactic.cpp:57-156.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
)

type fmBoundKind uint8

const (
	fmNoBound fmBoundKind = iota
	fmLowerBound
	fmUpperBound
)

func fmRat(m *ast.ASTManager, h ast.ASTHandle) (*big.Rat, bool, error) {
	s, _, yes, e := m.ArithmeticNumeralText(h)
	if e != nil || !yes {
		return nil, yes, e
	}
	v, ok := new(big.Rat).SetString(s)
	return v, ok, nil
}
func (c *FMModelConverter) evaluate(ev *model.ModelEvaluator, h ast.ASTHandle, ref ast.ASTRef) (ast.ASTHandle, error) {
	if e := ev.Evaluate(h, ref); e != nil {
		return ast.ASTHandle{}, e
	}
	return c.manager.RefGet(ref)
}
func (c *FMModelConverter) processBound(variable, clause ast.ASTHandle, ev *model.ModelEvaluator, ref ast.ASTRef) (fmBoundKind, *big.Rat, error) {
	m := c.manager
	f, op, n, e := c.appInfo(clause)
	if e != nil {
		return fmNoBound, nil, e
	}
	lits := []ast.ASTHandle{clause}
	if f == m.BasicFamilyID() && op == ast.ASTOpOr {
		lits = nil
		for i := uint32(0); i < n; i++ {
			h, e := m.GetArg(clause, i)
			if e != nil {
				return fmNoBound, nil, e
			}
			lits = append(lits, h)
		}
	}
	var kind fmBoundKind
	var result *big.Rat
	for _, lit := range lits {
		neg, arg, e := c.notArg(lit)
		if e != nil {
			return fmNoBound, nil, e
		}
		atom := lit
		if neg {
			atom = arg
		}
		if yes, _ := c.uninterpConst(atom); yes {
			value, e := c.evaluate(ev, lit, ref)
			if e != nil {
				return fmNoBound, nil, e
			}
			truth, e := m.MkTrue()
			if e != nil {
				return fmNoBound, nil, e
			}
			if value == truth {
				return fmNoBound, nil, nil
			}
			continue
		}
		af, aop, an, e := c.appInfo(atom)
		if e != nil {
			return fmNoBound, nil, e
		}
		arith, e := m.ArithmeticFamilyID()
		if e != nil {
			return fmNoBound, nil, e
		}
		if af != arith || (aop != 2 && aop != 3) || an != 2 {
			return fmNoBound, nil, ast.ErrASTDomain
		}
		lhs, e := m.GetArg(atom, 0)
		if e != nil {
			return fmNoBound, nil, e
		}
		rhs, e := m.GetArg(atom, 1)
		if e != nil {
			return fmNoBound, nil, e
		}
		bound, ok, e := fmRat(m, rhs)
		if e != nil || !ok {
			return fmNoBound, nil, e
		}
		strict := neg
		directionNeg := neg
		if aop == 3 {
			directionNeg = !directionNeg
		}
		if directionNeg {
			bound.Neg(bound)
		}
		terms := []ast.ASTHandle{lhs}
		lf, lop, ln, _ := c.appInfo(lhs)
		if lf == arith && lop == 6 {
			terms = nil
			for i := uint32(0); i < ln; i++ {
				h, e := m.GetArg(lhs, i)
				if e != nil {
					return fmNoBound, nil, e
				}
				terms = append(terms, h)
			}
		}
		coefficient := new(big.Rat)
		found := false
		for _, term := range terms {
			v := term
			coef := big.NewRat(1, 1)
			tf, top, tn, _ := c.appInfo(term)
			if tf == arith && top == 9 && tn == 2 {
				first, e := m.GetArg(term, 0)
				if e != nil {
					return fmNoBound, nil, e
				}
				second, e := m.GetArg(term, 1)
				if e != nil {
					return fmNoBound, nil, e
				}
				if q, yes, e := fmRat(m, first); e != nil {
					return fmNoBound, nil, e
				} else if yes {
					coef = q
					v = second
				}
			}
			// Source strips to_real for declaration identity, while evaluating the
			// original monomial for variables already interpreted in the model.
			vf, vop, vn, e := c.appInfo(v)
			if e != nil {
				return fmNoBound, nil, e
			}
			if vf == arith && vop == 14 && vn == 1 {
				v, e = m.GetArg(v, 0)
				if e != nil {
					return fmNoBound, nil, e
				}
			}
			if ast.ASTSourceDebug {
				yes, e := c.uninterpConst(v)
				if e != nil {
					return fmNoBound, nil, e
				}
				if !yes {
					return fmNoBound, nil, ast.ErrASTDomain
				}
			}
			if yes, _ := c.isVariable(v, variable); yes {
				coefficient.Set(coef)
				if directionNeg {
					coefficient.Neg(coefficient)
				}
				found = true
				continue
			}
			value, e := c.evaluate(ev, term, ref)
			if e != nil {
				return fmNoBound, nil, e
			}
			q, yes, e := fmRat(m, value)
			if ast.ASTSourceDebug && e == nil && !yes {
				return fmNoBound, nil, ast.ErrASTDomain
			}
			if e != nil || !yes {
				return fmNoBound, nil, e
			}
			if directionNeg {
				q.Neg(q)
			}
			bound.Sub(bound, q)
		}
		if !found || coefficient.Sign() == 0 {
			return fmNoBound, nil, ast.ErrASTDomain
		}
		sort, e := m.GetRange(variable)
		if e != nil {
			return fmNoBound, nil, e
		}
		integer, e := m.GetDeclKind(sort)
		if e != nil {
			return fmNoBound, nil, e
		}
		if integer == 1 && strict {
			if ast.ASTSourceDebug && !bound.IsInt() {
				return fmNoBound, nil, ast.ErrASTDomain
			}
			bound.Sub(bound, big.NewRat(1, 1))
		}
		lower := coefficient.Sign() < 0
		bound.Quo(bound, coefficient)
		if integer == 1 {
			if lower {
				bound = fmCeil(bound)
			} else {
				bound = fmFloor(bound)
			}
		}
		if lower {
			kind = fmLowerBound
		} else {
			kind = fmUpperBound
		}
		result = bound
	}
	if ast.ASTSourceDebug && kind == fmNoBound {
		return fmNoBound, nil, ast.ErrASTDomain
	}
	return kind, result, nil
}
func fmFloor(v *big.Rat) *big.Rat {
	q := new(big.Int).Quo(v.Num(), v.Denom())
	if v.Sign() < 0 && new(big.Int).Rem(v.Num(), v.Denom()).Sign() != 0 {
		q.Sub(q, big.NewInt(1))
	}
	return new(big.Rat).SetInt(q)
}
func fmCeil(v *big.Rat) *big.Rat {
	q := new(big.Int).Quo(v.Num(), v.Denom())
	if v.Sign() > 0 && new(big.Int).Rem(v.Num(), v.Denom()).Sign() != 0 {
		q.Add(q, big.NewInt(1))
	}
	return new(big.Rat).SetInt(q)
}
