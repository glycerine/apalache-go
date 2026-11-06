package arith

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (i *rootObjectInvocation) process(value *polynomial.Algebraic, k ast.ASTHandle, entry *rootObjectEntry) error {
	return processRootObjectConstraints(i.owner.manager, i.owner.arithmetic, value, k, func(h ast.ASTHandle) error { return i.add(h, entry) })
}

func processRootObjectConstraints(m *ast.ASTManager, arithmetic ast.FamilyID, value *polynomial.Algebraic, k ast.ASTHandle, add func(ast.ASTHandle) error) (err error) {
	am, e := m.ArithmeticAlgebraicManager()
	if e != nil {
		return e
	}
	coeff, err := am.DefiningPolynomialText(value)
	if err != nil {
		return err
	}
	lower, upper, err := am.IrrationalBoundsText(value)
	if err != nil {
		return err
	}
	if len(coeff) <= 2 {
		return ast.ErrASTDomain
	}
	t := rootObjectTemps{manager: m}
	defer func() { err = errors.Join(err, t.close()) }()
	terms := make([]ast.ASTHandle, 0, len(coeff))
	for degree, text := range coeff {
		if text == "0" {
			continue
		}
		coefficient, e := m.ArithmeticRealNumeralText(text)
		if e != nil {
			return e
		}
		if _, e = t.hold(coefficient); e != nil {
			return e
		}
		term := coefficient
		if degree > 0 {
			power := k
			if degree > 1 {
				exponent, e := m.ArithmeticRealNumeralText(fmt.Sprint(degree))
				if e != nil {
					return e
				}
				if _, e = t.hold(exponent); e != nil {
					return e
				}
				power, e = m.MkArithmeticPower(k, exponent)
				if e != nil {
					return e
				}
				if _, e = t.hold(power); e != nil {
					return e
				}
			}
			term, e = m.MkArithmeticMul(coefficient, power)
			if e != nil {
				return e
			}
			if _, e = t.hold(term); e != nil {
				return e
			}
		}
		terms = append(terms, term)
	}
	if len(terms) < 2 {
		return ast.ErrASTDomain
	}
	sum, e := m.MkFamilyApp(arithmetic, 6, nil, terms)
	if e != nil {
		return e
	}
	if _, e = t.hold(sum); e != nil {
		return e
	}
	zero, e := m.ArithmeticRealNumeralText("0")
	if e != nil {
		return e
	}
	if _, e = t.hold(zero); e != nil {
		return e
	}
	eq, e := m.MkEq(sum, zero)
	if e != nil {
		return e
	}
	if e = add(eq); e != nil {
		return e
	}
	lo, e := m.ArithmeticRealNumeralText(lower)
	if e != nil {
		return e
	}
	if _, e = t.hold(lo); e != nil {
		return e
	}
	lowerBound, e := m.MkArithmeticLT(lo, k)
	if e != nil {
		return e
	}
	if e = add(lowerBound); e != nil {
		return e
	}
	hi, e := m.ArithmeticRealNumeralText(upper)
	if e != nil {
		return e
	}
	if _, e = t.hold(hi); e != nil {
		return e
	}
	upperBound, e := m.MkArithmeticLT(k, hi)
	if e != nil {
		return e
	}
	return add(upperBound)
}
