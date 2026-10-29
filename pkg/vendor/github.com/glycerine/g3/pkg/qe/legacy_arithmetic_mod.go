package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::mod2div.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *legacyArithmeticProjection) modToDivisibility(formula ast.ASTHandle) (ast.ASTRef, error) {
	if p == nil || p.syntax == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	var rewrite func(ast.ASTHandle) (ast.ASTHandle, error)
	rewrite = func(current ast.ASTHandle) (ast.ASTHandle, error) {
		if cached, ok := memo[current]; ok {
			return cached, nil
		}
		kind, err := p.syntax.manager.GetKind(current)
		if err != nil || kind != ast.ASTApp {
			return current, err
		}
		args, equality, err := p.syntax.operation(current, p.syntax.manager.BasicFamilyID(), ast.ASTOpEq)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if equality && len(args) == 2 {
			for side := 0; side < 2; side++ {
				mod, modular, err := p.syntax.operation(args[side], p.syntax.family, 13)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if !modular || len(mod) != 2 {
					continue
				}
				divisor, numeral, err := p.syntax.numeral(mod[1])
				if err != nil {
					return ast.ASTHandle{}, err
				}
				integer, arithmetic, err := p.syntax.arithmetic(mod[1])
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if !numeral || !arithmetic || !integer {
					continue
				}
				left, err := rewrite(mod[0])
				if err != nil {
					return ast.ASTHandle{}, err
				}
				right, err := rewrite(args[1-side])
				if err != nil {
					return ast.ASTHandle{}, err
				}
				zero, err := p.syntax.manager.ArithmeticIntegerNumeralText("0")
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if value, isNumeral, err := p.syntax.numeral(right); err != nil {
					return ast.ASTHandle{}, err
				} else if isNumeral && value.Sign() == 0 {
					modulus, err := p.syntax.manager.MkFamilyApp(p.syntax.family, 13, nil, []ast.ASTHandle{left, mod[1]})
					if err != nil {
						return ast.ASTHandle{}, err
					}
					result, err := p.syntax.manager.MkEq(modulus, zero)
					if err == nil {
						memo[current] = result
					}
					return result, err
				}
				difference, err := p.syntax.manager.MkArithmeticSub(left, right)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				modulus, err := p.syntax.manager.MkFamilyApp(p.syntax.family, 13, nil, []ast.ASTHandle{difference, mod[1]})
				if err != nil {
					return ast.ASTHandle{}, err
				}
				divides, err := p.syntax.manager.MkEq(modulus, zero)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				lower, err := p.syntax.manager.MkArithmeticLE(zero, right)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				absolute, err := p.syntax.manager.ArithmeticNumeralRat(new(big.Rat).Abs(divisor), true)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				upper, err := p.syntax.manager.MkFamilyApp(p.syntax.family, 4, nil, []ast.ASTHandle{right, absolute})
				if err != nil {
					return ast.ASTHandle{}, err
				}
				result, err := p.syntax.manager.MkAndArray([]ast.ASTHandle{divides, lower, upper})
				if err == nil {
					memo[current] = result
				}
				return result, err
			}
		}
		count, err := p.syntax.manager.GetNumArgs(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		children := make([]ast.ASTHandle, count)
		for index := range children {
			child, err := p.syntax.manager.GetArg(current, uint32(index))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			children[index], err = rewrite(child)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		decl, err := p.syntax.manager.GetDecl(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		result, err := p.syntax.manager.MkApp(decl, children)
		if err == nil {
			memo[current] = result
		}
		return result, err
	}
	result, err := rewrite(formula)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.syntax.manager.NewRef(result)
}
