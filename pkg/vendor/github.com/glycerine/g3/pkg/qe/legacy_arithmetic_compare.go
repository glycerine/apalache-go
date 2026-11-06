package qe

// Source: src/muz/spacer/spacer_qe_project.cpp
// arith_project_util::mk_lt, mk_le and mk_eq.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *legacyArithmeticProjection) rewriteLegacy(expression ast.ASTHandle) (_ ast.ASTRef, resultErr error) {
	if p == nil || p.syntax == nil || p.theory == nil {
		return ast.ASTRef{}, ast.ErrASTHandle
	}
	result, err := p.syntax.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTRef{}, err
	}
	if err = p.theory.Rewrite(expression, result); err != nil {
		return ast.ASTRef{}, errors.Join(err, p.syntax.manager.DelRef(&result))
	}
	return result, nil
}

func (p *legacyArithmeticProjection) multiplyLegacy(coefficient *big.Rat, term ast.ASTHandle) (ast.ASTHandle, error) {
	integer, arithmetic, err := p.syntax.arithmetic(term)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !arithmetic {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return p.scaledTerm(coefficient, term, integer)
}

// oppositeBound eliminates x from bounds whose coefficients have opposite
// signs. Strictness is inherited from either input bound.
func (p *legacyArithmeticProjection) oppositeBound(left, right *legacyLinearLiteral) (ast.ASTRef, error) {
	if left == nil || right == nil || left.manager != p.syntax.manager || right.manager != p.syntax.manager || left.coefficient.Sign()*right.coefficient.Sign() >= 0 {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	leftTerm, err := p.syntax.manager.RefGet(left.term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	rightTerm, err := p.syntax.manager.RefGet(right.term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	weightedLeft, err := p.multiplyLegacy(new(big.Rat).Abs(right.coefficient), leftTerm)
	if err != nil {
		return ast.ASTRef{}, err
	}
	weightedRight, err := p.multiplyLegacy(new(big.Rat).Abs(left.coefficient), rightTerm)
	if err != nil {
		return ast.ASTRef{}, err
	}
	sum, err := p.syntax.manager.MkArithmeticAdd(weightedLeft, weightedRight)
	if err != nil {
		return ast.ASTRef{}, err
	}
	integer, _, err := p.syntax.arithmetic(leftTerm)
	if err != nil {
		return ast.ASTRef{}, err
	}
	zero, err := p.syntax.manager.ArithmeticNumeralRat(new(big.Rat), integer)
	if err != nil {
		return ast.ASTRef{}, err
	}
	comparison := ast.ASTHandle{}
	if left.strict || right.strict {
		comparison, err = p.syntax.manager.MkFamilyApp(p.syntax.family, 4, nil, []ast.ASTHandle{sum, zero})
	} else {
		comparison, err = p.syntax.manager.MkArithmeticLE(sum, zero)
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.rewriteLegacy(comparison)
}

// sameBound orders two bounds from the same side. It is strict exactly when
// the left bound is strict and the selected right bound is not.
func (p *legacyArithmeticProjection) sameBound(left, right *legacyLinearLiteral) (ast.ASTRef, error) {
	if left == nil || right == nil || left.manager != p.syntax.manager || right.manager != p.syntax.manager || left.coefficient.Sign() == 0 || left.coefficient.Sign()*right.coefficient.Sign() <= 0 {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	leftTerm, err := p.syntax.manager.RefGet(left.term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	rightTerm, err := p.syntax.manager.RefGet(right.term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	weightedLeft, err := p.multiplyLegacy(new(big.Rat).Abs(right.coefficient), leftTerm)
	if err != nil {
		return ast.ASTRef{}, err
	}
	weightedRight, err := p.multiplyLegacy(new(big.Rat).Abs(left.coefficient), rightTerm)
	if err != nil {
		return ast.ASTRef{}, err
	}
	comparison := ast.ASTHandle{}
	if left.strict && !right.strict {
		comparison, err = p.syntax.manager.MkFamilyApp(p.syntax.family, 4, nil, []ast.ASTHandle{weightedLeft, weightedRight})
	} else {
		comparison, err = p.syntax.manager.MkArithmeticLE(weightedLeft, weightedRight)
	}
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.rewriteLegacy(comparison)
}

func (p *legacyArithmeticProjection) equalityAtBound(equality, bound *legacyLinearLiteral) (ast.ASTRef, error) {
	if equality == nil || bound == nil || equality.manager != p.syntax.manager || bound.manager != p.syntax.manager || equality.coefficient.Sign() == 0 || bound.coefficient.Sign() == 0 {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	equalityTerm, err := p.syntax.manager.RefGet(equality.term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	boundTerm, err := p.syntax.manager.RefGet(bound.term)
	if err != nil {
		return ast.ASTRef{}, err
	}
	left, err := p.multiplyLegacy(equality.coefficient, boundTerm)
	if err != nil {
		return ast.ASTRef{}, err
	}
	right, err := p.multiplyLegacy(bound.coefficient, equalityTerm)
	if err != nil {
		return ast.ASTRef{}, err
	}
	result, err := p.syntax.manager.MkEq(left, right)
	if err != nil {
		return ast.ASTRef{}, err
	}
	return p.rewriteLegacy(result)
}
