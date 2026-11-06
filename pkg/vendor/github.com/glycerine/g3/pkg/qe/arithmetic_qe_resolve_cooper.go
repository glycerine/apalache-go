package qe

// Source: src/qe/qe_arith_plugin.cpp, mk_resolve and mk_big_or.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// resolveCooper handles the source's explicit small enumeration and,
// when a QE context is available, its symbolic bit-vector expansion.
func (u *ArithmeticQETermUtil) resolveCooper(variable ast.ASTHandle, a *big.Rat, t ast.ASTHandle, b *big.Rat, s ast.ASTHandle) (result ast.ASTRef, handled bool, err error) {
	absA := new(big.Rat).Abs(a)
	absB := new(big.Rat).Abs(b)
	if !absA.IsInt() || !absB.IsInt() {
		return result, false, ast.ErrASTDomain
	}
	if absA.Cmp(absB) < 0 {
		absA, absB = absB, absA
		a, b = b, a
		t, s = s, t
	}
	if absB.Cmp(big.NewRat(3, 1)) > 0 && u.qeConfig == nil {
		return result, false, nil
	}
	m := u.manager
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			err = errors.Join(err, m.DelRef(&refs[i]))
		}
		if err != nil && result != (ast.ASTRef{}) {
			err = errors.Join(err, m.DelRef(&result))
		}
	}()
	hold := func(h ast.ASTHandle, cause error) (ast.ASTHandle, error) {
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		ref, cause := m.NewRef(h)
		if cause == nil {
			refs = append(refs, ref)
		}
		return h, cause
	}
	own := func(ref ast.ASTRef, cause error) (ast.ASTHandle, error) {
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		refs = append(refs, ref)
		return m.RefGet(ref)
	}
	bt, err := own(u.scale(absB, t))
	if err != nil {
		return result, false, err
	}
	as, err := own(u.scale(absA, s))
	if err != nil {
		return result, false, err
	}
	sum, err := hold(m.MkArithmeticAdd(as, bt))
	if err != nil {
		return result, false, err
	}
	base, err := own(u.comparisonZero(sum, false))
	if err != nil {
		return result, false, err
	}
	slack := new(big.Rat).Mul(new(big.Rat).Sub(absA, big.NewRat(1, 1)), new(big.Rat).Sub(absB, big.NewRat(1, 1)))
	slackValue, err := hold(m.ArithmeticNumeralRat(slack, true))
	if err != nil {
		return result, false, err
	}
	withSlack, err := hold(m.MkArithmeticAdd(sum, slackValue))
	if err != nil {
		return result, false, err
	}
	first, err := own(u.comparisonZero(withSlack, false))
	if err != nil {
		return result, false, err
	}
	sz, err := hold(m.MkArithmeticAdd(s, variable))
	if err != nil {
		return result, false, err
	}
	if b.Sign() > 0 {
		sz, err = hold(m.MkArithmeticNeg(sz))
		if err != nil {
			return result, false, err
		}
	}
	scaled, err := own(u.scale(a, sz))
	if err != nil {
		return result, false, err
	}
	boundTerm, err := hold(m.MkArithmeticAdd(scaled, bt))
	if err != nil {
		return result, false, err
	}
	bound, err := own(u.comparisonZero(boundTerm, false))
	if err != nil {
		return result, false, err
	}
	divides, err := own(u.MakeDivides(absB, sz))
	if err != nil {
		return result, false, err
	}
	conjunction, err := hold(m.MkAnd(divides, bound))
	if err != nil {
		return result, false, err
	}
	replacer, err := rewriter.NewExprSafeReplace(m)
	if err != nil {
		return result, false, err
	}
	defer func() { err = errors.Join(err, replacer.Close()) }()
	var cases ast.ASTHandle
	if absB.Cmp(big.NewRat(3, 1)) <= 0 {
		var alternatives []ast.ASTHandle
		limit := absB.Num().Int64() - 2
		for i := int64(0); i <= limit; i++ {
			numeral, e := hold(m.ArithmeticNumeralRat(big.NewRat(i, 1), true))
			if e != nil {
				return result, false, e
			}
			ref, e := m.NewRef(conjunction)
			if e != nil {
				return result, false, e
			}
			refs = append(refs, ref)
			if e = replacer.ApplySubstitution(variable, numeral, ref); e != nil {
				return result, false, e
			}
			alternative, e := m.RefGet(ref)
			if e != nil {
				return result, false, e
			}
			alternatives = append(alternatives, alternative)
		}
		cases, err = hold(m.MkOrArgs(alternatives))
	} else {
		upper := new(big.Rat).Sub(absB, big.NewRat(2, 1))
		upperValue, e := hold(m.ArithmeticNumeralRat(upper, true))
		if e != nil {
			return result, false, e
		}
		bound, e := hold(m.MkArithmeticLE(variable, upperValue))
		if e != nil {
			return result, false, e
		}
		body, e := hold(m.MkAnd(bound, conjunction))
		if e != nil {
			return result, false, e
		}
		width := uint32(upper.Num().BitLen())
		if width == 0 {
			width = 1
		}
		sort, e := hold(m.BitVectorSort(width))
		if e != nil {
			return result, false, e
		}
		bits, e := hold(m.MkFreshConst("z", sort))
		if e != nil {
			return result, false, e
		}
		residue, e := hold(m.MkBitVectorToInt(bits))
		if e != nil {
			return result, false, e
		}
		rewritten, e := m.NewRef(body)
		if e != nil {
			return result, false, e
		}
		refs = append(refs, rewritten)
		if e = replacer.ApplySubstitution(variable, residue, rewritten); e != nil {
			return result, false, e
		}
		body, e = m.RefGet(rewritten)
		if e != nil {
			return result, false, e
		}
		quantifier, e := bindExprQuantifierVariables(m, []ast.ASTHandle{bits}, body)
		if e != nil {
			return result, false, e
		}
		refs = append(refs, quantifier)
		q, e := m.RefGet(quantifier)
		if e != nil {
			return result, false, e
		}
		blasted, complete, e := eliminateQuantifierBitVectorSearch(m, q, *u.qeConfig)
		if e != nil || !complete {
			return result, false, e
		}
		refs = append(refs, blasted)
		cases, err = m.RefGet(blasted)
	}
	if err != nil {
		return result, false, err
	}
	second, err := hold(m.MkAnd(base, cases))
	if err != nil {
		return result, false, err
	}
	answer, err := m.MkOr(first, second)
	if err != nil {
		return result, false, err
	}
	result, err = m.NewRef(answer)
	return result, err == nil, err
}
