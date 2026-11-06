package arith

// Source: src/tactic/arith/purify_arith_tactic.cpp process_to_int.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (i *qfnraPurifyInvocation) processToInt(x, fresh ast.ASTHandle, entry *qfnraPurifyEntry) (err error) {
	m := i.owner.manager
	temps := qfnraPurifyTemps{manager: m}
	defer func() { err = errors.Join(err, temps.close()) }()
	toReal, err := m.MkFamilyApp(i.owner.arithmetic, 14, nil, []ast.ASTHandle{fresh})
	if err != nil {
		return err
	}
	if _, err = temps.hold(toReal); err != nil {
		return err
	}
	minusOne, err := m.ArithmeticRealNumeralText("-1")
	if err != nil {
		return err
	}
	if _, err = temps.hold(minusOne); err != nil {
		return err
	}
	product, err := m.MkArithmeticMul(minusOne, toReal)
	if err != nil {
		return err
	}
	if _, err = temps.hold(product); err != nil {
		return err
	}
	diff, err := m.MkArithmeticAdd(x, product)
	if err != nil {
		return err
	}
	if _, err = temps.hold(diff); err != nil {
		return err
	}
	zero, err := m.ArithmeticRealNumeralText("0")
	if err != nil {
		return err
	}
	if _, err = temps.hold(zero); err != nil {
		return err
	}
	lower, err := m.MkArithmeticGE(diff, zero)
	if err != nil {
		return err
	}
	if err = i.addConstraint(lower, entry); err != nil {
		return err
	}
	one, err := m.ArithmeticRealNumeralText("1")
	if err != nil {
		return err
	}
	if _, err = temps.hold(one); err != nil {
		return err
	}
	upper, err := m.MkArithmeticGE(diff, one)
	if err != nil {
		return err
	}
	if _, err = temps.hold(upper); err != nil {
		return err
	}
	below, err := m.MkNot(upper)
	if err != nil {
		return err
	}
	return i.addConstraint(below, entry)
}
