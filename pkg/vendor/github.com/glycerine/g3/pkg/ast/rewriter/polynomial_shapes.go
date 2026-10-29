package rewriter

// Source: src/ast/rewriter/poly_rewriter.h:121-122;
// src/ast/rewriter/poly_rewriter_def.h:980-1032.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// IsTimesMinusOne recognizes the arithmetic polynomial shape (* -1 term).
func (r *ArithRewriter) IsTimesMinusOne(n ast.ASTHandle) (term ast.ASTHandle, matched bool, err error) {
	if err = r.open(); err != nil {
		return term, false, err
	}
	kind, err := r.manager.GetKind(n)
	if err != nil || kind != ast.ASTApp {
		return term, false, err
	}
	_, args, product, err := r.operation(n, arithOpMul)
	if err != nil || !product || len(args) != 2 {
		return term, false, err
	}
	coefficient, _, numeral, err := r.numeral(args[0])
	if err != nil || !numeral || coefficient.Cmp(big.NewRat(-1, 1)) != 0 {
		return term, false, err
	}
	return args[1], true, nil
}

// IsVarPlusGround recognizes the source's right-associated arithmetic sum
// (+ ground ... (+ [(- 1 variable)|variable] ground)).
func (r *ArithRewriter) IsVarPlusGround(n ast.ASTHandle, out ast.ASTRef) (inverse bool, variable ast.ASTHandle, matched bool, err error) {
	if err = r.open(); err != nil {
		return false, variable, false, err
	}
	kind, err := r.manager.GetKind(n)
	if err != nil || kind != ast.ASTApp {
		return false, variable, false, err
	}
	_, _, sum, err := r.operation(n, arithOpAdd)
	if err != nil || !sum {
		return false, variable, false, err
	}
	closed, err := r.manager.IsGround(n)
	if err != nil || closed {
		return false, variable, false, err
	}

	curr := n
	groundTerms := make([]ast.ASTHandle, 0, 2)
	for {
		arg := curr
		kind, e := r.manager.GetKind(curr)
		if e != nil {
			return false, variable, false, e
		}
		var args []ast.ASTHandle
		add := false
		if kind == ast.ASTApp {
			_, args, add, e = r.operation(curr, arithOpAdd)
			if e != nil {
				return false, variable, false, e
			}
		}
		if add {
			if len(args) < 2 {
				return false, variable, false, ast.ErrASTDomain
			}
			arg, curr = args[0], args[1]
		}
		isGround, e := r.manager.IsGround(arg)
		if e != nil {
			return false, variable, false, e
		}
		if isGround {
			groundTerms = append(groundTerms, arg)
		} else {
			kind, e := r.manager.GetKind(arg)
			if e != nil {
				return false, variable, false, e
			}
			candidate, negated := arg, false
			if kind != ast.ASTVar {
				candidate, negated, e = r.IsTimesMinusOne(arg)
				if e != nil {
					return false, variable, false, e
				}
				if negated {
					kind, e = r.manager.GetKind(candidate)
					if e != nil {
						return false, variable, false, e
					}
				}
			}
			if kind != ast.ASTVar || variable != (ast.ASTHandle{}) {
				return false, variable, false, nil
			}
			variable, inverse = candidate, negated
		}
		if !add {
			break
		}
	}
	if variable == (ast.ASTHandle{}) || len(groundTerms) == 0 {
		return false, ast.ASTHandle{}, false, nil
	}
	status, err := r.rewriteAdd(ast.SliceArguments(groundTerms), out)
	if err != nil {
		return false, ast.ASTHandle{}, false, err
	}
	if status == RewriteFailed {
		ground, e := r.makeAdd(groundTerms, n)
		if e != nil {
			return false, ast.ASTHandle{}, false, e
		}
		if e = r.manager.SetRef(out, ground); e != nil {
			return false, ast.ASTHandle{}, false, e
		}
	}
	return inverse, variable, true, nil
}

// IsTimesMinusOne recognizes the bit-vector polynomial shape (bvmul -1 term).
func (p *BVPolynomialRewriter) IsTimesMinusOne(n ast.ASTHandle) (term ast.ASTHandle, matched bool, err error) {
	if err = p.open(); err != nil {
		return term, false, err
	}
	product, err := p.manager.IsBitVectorOperation(n, 6)
	if err != nil || !product {
		return term, false, err
	}
	count, err := p.manager.GetNumArgs(n)
	if err != nil || count != 2 {
		return term, false, err
	}
	coefficient, err := p.manager.GetArg(n, 0)
	if err != nil {
		return term, false, err
	}
	minusOne, err := p.manager.IsBitVectorAllOne(coefficient)
	if err != nil || !minusOne {
		return term, false, err
	}
	term, err = p.manager.GetArg(n, 1)
	return term, err == nil, err
}

// IsVarPlusGround is the concrete bit-vector specialization of the source
// right-associated polynomial-shape recognizer.
func (p *BVPolynomialRewriter) IsVarPlusGround(n ast.ASTHandle, out ast.ASTRef) (inverse bool, variable ast.ASTHandle, matched bool, err error) {
	if err = p.open(); err != nil {
		return false, variable, false, err
	}
	add, err := p.manager.IsBitVectorOperation(n, 4)
	if err != nil || !add {
		return false, variable, false, err
	}
	closed, err := p.manager.IsGround(n)
	if err != nil || closed {
		return false, variable, false, err
	}

	curr := n
	groundTerms := make([]ast.ASTHandle, 0, 2)
	for {
		arg := curr
		add, e := p.manager.IsBitVectorOperation(curr, 4)
		if e != nil {
			return false, variable, false, e
		}
		if add {
			count, e := p.manager.GetNumArgs(curr)
			if e != nil {
				return false, variable, false, e
			}
			if count < 2 {
				return false, variable, false, ast.ErrASTDomain
			}
			arg, e = p.manager.GetArg(curr, 0)
			if e != nil {
				return false, variable, false, e
			}
			curr, e = p.manager.GetArg(curr, 1)
			if e != nil {
				return false, variable, false, e
			}
		}
		isGround, e := p.manager.IsGround(arg)
		if e != nil {
			return false, variable, false, e
		}
		if isGround {
			groundTerms = append(groundTerms, arg)
		} else {
			kind, e := p.manager.GetKind(arg)
			if e != nil {
				return false, variable, false, e
			}
			candidate, negated := arg, false
			if kind != ast.ASTVar {
				candidate, negated, e = p.IsTimesMinusOne(arg)
				if e != nil {
					return false, variable, false, e
				}
				if negated {
					kind, e = p.manager.GetKind(candidate)
					if e != nil {
						return false, variable, false, e
					}
				}
			}
			if kind != ast.ASTVar || variable != (ast.ASTHandle{}) {
				return false, variable, false, nil
			}
			variable, inverse = candidate, negated
		}
		if !add {
			break
		}
	}
	if variable == (ast.ASTHandle{}) || len(groundTerms) == 0 {
		return false, ast.ASTHandle{}, false, nil
	}
	if err = p.MakeAddition(ast.SliceArguments(groundTerms), out); err != nil {
		return false, ast.ASTHandle{}, false, err
	}
	return inverse, variable, true, nil
}
