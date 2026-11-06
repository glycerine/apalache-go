package qe

// Source: src/qe/qe_arith_plugin.cpp bounds_proc inequality collection.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ArithmeticBoundsProcessor) normalizeRest(variable *ast.ContainsApp, coefficient *big.Rat, rest ast.ASTRef) (*big.Rat, ast.ASTRef, error) {
	target, err := variable.Target()
	if err != nil {
		return nil, rest, err
	}
	integer, err := p.util.isInteger(target)
	if err != nil || integer {
		return new(big.Rat).Set(coefficient), rest, err
	}
	restHandle, err := p.manager.RefGet(rest)
	if err != nil {
		return nil, rest, err
	}
	abs := new(big.Rat).Abs(coefficient)
	denominator, err := p.util.numeralFor(abs, restHandle)
	if err != nil {
		return nil, rest, err
	}
	divided, err := p.manager.MkArithmeticDiv(restHandle, denominator)
	if err != nil {
		return nil, rest, err
	}
	normalized, err := p.manager.NewRef(divided)
	if err != nil {
		return nil, rest, err
	}
	if err = p.manager.DelRef(&rest); err != nil {
		return nil, normalized, err
	}
	sign := big.NewRat(1, 1)
	if coefficient.Sign() < 0 {
		sign.Neg(sign)
	}
	return sign, normalized, nil
}

func (p *ArithmeticBoundsProcessor) appendBound(atom ast.ASTHandle, term ast.ASTRef, coefficient *big.Rat, strict bool) error {
	atomRef, err := p.manager.NewRef(atom)
	if err != nil {
		return errors.Join(err, p.manager.DelRef(&term))
	}
	direction := ArithmeticUpperBound
	if coefficient.Sign() < 0 {
		direction = ArithmeticLowerBound
	}
	p.bounds = append(p.bounds, &ArithmeticBound{manager: p.manager, Term: term, Atom: atomRef,
		Coefficient: new(big.Rat).Set(coefficient), Strict: strict, Direction: direction})
	return nil
}

func (p *ArithmeticBoundsProcessor) addNonStrict(variable *ast.ContainsApp, atom ast.ASTHandle) (bool, error) {
	polynomial, relation, err := p.util.MatchLE(atom)
	if err != nil || !relation {
		return false, err
	}
	polynomialHandle, err := p.manager.RefGet(polynomial)
	if err != nil {
		return false, errors.Join(err, p.manager.DelRef(&polynomial))
	}
	coefficient, rest, matched, err := p.util.Coefficient(variable, polynomialHandle)
	cleanup := p.manager.DelRef(&polynomial)
	if err != nil || cleanup != nil || !matched {
		if rest != (ast.ASTRef{}) {
			cleanup = errors.Join(cleanup, p.manager.DelRef(&rest))
		}
		return false, errors.Join(err, cleanup)
	}
	coefficient, rest, err = p.normalizeRest(variable, coefficient, rest)
	if err != nil {
		if rest != (ast.ASTRef{}) {
			err = errors.Join(err, p.manager.DelRef(&rest))
		}
		return false, err
	}
	if err = p.appendBound(atom, rest, coefficient, false); err != nil {
		return false, err
	}
	return true, nil
}

func (p *ArithmeticBoundsProcessor) addStrict(variable *ast.ContainsApp, atom ast.ASTHandle) (bool, error) {
	polynomial, relation, err := p.util.MatchLT(atom)
	if err != nil {
		return false, err
	}
	if !relation {
		inner, negative, cause := p.util.Negated(atom)
		if cause != nil || !negative {
			return false, cause
		}
		polynomial, relation, err = p.util.MatchGE(inner)
		if err != nil || !relation {
			return false, err
		}
	}
	target, err := variable.Target()
	if err != nil {
		return false, errors.Join(err, p.manager.DelRef(&polynomial))
	}
	integer, err := p.util.isInteger(target)
	if err != nil || integer {
		return false, errors.Join(err, p.manager.DelRef(&polynomial))
	}
	polynomialHandle, err := p.manager.RefGet(polynomial)
	if err != nil {
		return false, errors.Join(err, p.manager.DelRef(&polynomial))
	}
	coefficient, rest, matched, err := p.util.Coefficient(variable, polynomialHandle)
	cleanup := p.manager.DelRef(&polynomial)
	if err != nil || cleanup != nil || !matched {
		if rest != (ast.ASTRef{}) {
			cleanup = errors.Join(cleanup, p.manager.DelRef(&rest))
		}
		return false, errors.Join(err, cleanup)
	}
	coefficient, rest, err = p.normalizeRest(variable, coefficient, rest)
	if err != nil {
		if rest != (ast.ASTRef{}) {
			err = errors.Join(err, p.manager.DelRef(&rest))
		}
		return false, err
	}
	if err = p.appendBound(atom, rest, coefficient, true); err != nil {
		return false, err
	}
	return true, nil
}
