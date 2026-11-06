package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::project, integer coefficient/divisor normalization.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func legacyIntegerLCM(left, right *big.Rat) (*big.Rat, error) {
	if left == nil || right == nil || !left.IsInt() || !right.IsInt() {
		return nil, ast.ErrASTDomain
	}
	a := new(big.Int).Abs(left.Num())
	b := new(big.Int).Abs(right.Num())
	if a.Sign() == 0 || b.Sign() == 0 {
		return new(big.Rat), nil
	}
	gcd := new(big.Int).GCD(nil, nil, a, b)
	value := new(big.Int).Quo(a, gcd)
	value.Mul(value, b)
	return new(big.Rat).SetInt(value), nil
}

func (p *legacyArithmeticProjection) replaceLegacyTerm(literal *legacyLinearLiteral, term ast.ASTHandle) error {
	replacement, err := p.syntax.manager.NewRef(term)
	if err != nil {
		return err
	}
	if err = p.syntax.manager.DelRef(&literal.term); err != nil {
		_ = p.syntax.manager.DelRef(&replacement)
		return err
	}
	literal.term = replacement
	return nil
}

func (p *legacyArithmeticProjection) normalizeIntegers(literals []*legacyLinearLiteral) (lcmCoefficients, lcmDivisors *big.Rat, resultErr error) {
	if p == nil || p.syntax == nil {
		return nil, nil, ast.ErrASTHandle
	}
	lcmCoefficients = big.NewRat(1, 1)
	for _, literal := range literals {
		if literal == nil || literal.manager != p.syntax.manager || literal.coefficient.Sign() == 0 || !literal.coefficient.IsInt() {
			return nil, nil, ast.ErrASTDomain
		}
		lcmCoefficients, resultErr = legacyIntegerLCM(lcmCoefficients, literal.coefficient)
		if resultErr != nil {
			return nil, nil, resultErr
		}
	}
	lcmDivisors = big.NewRat(1, 1)
	for _, literal := range literals {
		factor := new(big.Rat).Quo(lcmCoefficients, new(big.Rat).Abs(literal.coefficient))
		if factor.Cmp(big.NewRat(1, 1)) != 0 {
			term, err := p.syntax.manager.RefGet(literal.term)
			if err != nil {
				return nil, nil, err
			}
			scaled, err := p.multiplyLegacy(factor, term)
			if err != nil {
				return nil, nil, err
			}
			if err = p.replaceLegacyTerm(literal, scaled); err != nil {
				return nil, nil, err
			}
		}
		if literal.coefficient.Sign() > 0 {
			literal.coefficient.Set(lcmCoefficients)
		} else {
			literal.coefficient.Neg(lcmCoefficients)
		}
		if literal.divisor.Sign() != 0 {
			literal.divisor.Mul(literal.divisor, factor)
			lcmDivisors, resultErr = legacyIntegerLCM(lcmDivisors, literal.divisor)
			if resultErr != nil {
				return nil, nil, resultErr
			}
		}
	}
	lcmDivisors, resultErr = legacyIntegerLCM(lcmDivisors, lcmCoefficients)
	if resultErr != nil {
		return nil, nil, resultErr
	}
	return lcmCoefficients, lcmDivisors, nil
}

func (p *legacyArithmeticProjection) substituteIntegerLiterals(literals []*legacyLinearLiteral, originals []ast.ASTHandle, xTerm ast.ASTHandle, selected int) (_ *legacyRealProjection, resultErr error) {
	if p == nil || p.syntax == nil || len(literals) != len(originals) || selected < 0 || selected >= len(literals) {
		return nil, ast.ErrASTHandle
	}
	result := &legacyRealProjection{manager: p.syntax.manager}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	zero, err := p.syntax.manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return nil, err
	}
	for index, literal := range literals {
		if literal == nil || literal.manager != p.syntax.manager {
			return nil, ast.ErrASTDomain
		}
		if index == selected {
			replacement, err := p.constantLegacy(true)
			if err != nil {
				return nil, err
			}
			p.appendLegacyReplacement(result, originals[index], replacement)
			continue
		}
		term, err := p.syntax.manager.RefGet(literal.term)
		if err != nil {
			return nil, err
		}
		var combined ast.ASTHandle
		if literal.coefficient.Sign() < 0 {
			combined, err = p.syntax.manager.MkArithmeticSub(term, xTerm)
		} else {
			combined, err = p.syntax.manager.MkArithmeticAdd(term, xTerm)
		}
		if err != nil {
			return nil, err
		}
		var replacement ast.ASTRef
		if literal.divisor.Sign() == 0 {
			var expression ast.ASTHandle
			switch {
			case literal.equality:
				expression, err = p.syntax.manager.MkEq(combined, zero)
			case literal.strict:
				expression, err = p.syntax.manager.MkFamilyApp(p.syntax.family, 4, nil, []ast.ASTHandle{combined, zero})
			default:
				expression, err = p.syntax.manager.MkArithmeticLE(combined, zero)
			}
			if err == nil {
				replacement, err = p.rewriteLegacy(expression)
			}
		} else {
			rewritten, rewriteErr := p.rewriteLegacy(combined)
			if rewriteErr != nil {
				return nil, rewriteErr
			}
			rewrittenHandle, rewriteErr := p.syntax.manager.RefGet(rewritten)
			if rewriteErr != nil {
				_ = p.syntax.manager.DelRef(&rewritten)
				return nil, rewriteErr
			}
			modulus, rewriteErr := p.syntax.manager.ArithmeticNumeralRat(literal.divisor, true)
			if rewriteErr == nil {
				var modular ast.ASTHandle
				modular, rewriteErr = p.syntax.manager.MkFamilyApp(p.syntax.family, 13, nil, []ast.ASTHandle{rewrittenHandle, modulus})
				if rewriteErr == nil {
					var expression ast.ASTHandle
					expression, rewriteErr = p.syntax.manager.MkEq(modular, zero)
					if rewriteErr == nil {
						replacement, rewriteErr = p.syntax.manager.NewRef(expression)
					}
				}
			}
			err = errors.Join(rewriteErr, p.syntax.manager.DelRef(&rewritten))
		}
		if err != nil {
			return nil, err
		}
		p.appendLegacyReplacement(result, originals[index], replacement)
	}
	return result, nil
}
