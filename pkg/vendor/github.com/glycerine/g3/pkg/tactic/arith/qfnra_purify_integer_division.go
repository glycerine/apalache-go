package arith

// Source: src/tactic/arith/purify_arith_tactic.cpp process_idiv/process_mod.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// qfnraPurifyBuilder roots intermediate ASTs until all constraints own them.
type qfnraPurifyBuilder struct {
	temps qfnraPurifyTemps
	err   error
}

func (b *qfnraPurifyBuilder) hold(build func() (ast.ASTHandle, error)) ast.ASTHandle {
	if b.err != nil {
		return ast.ASTHandle{}
	}
	value, err := build()
	if err != nil {
		b.err = err
		return ast.ASTHandle{}
	}
	value, b.err = b.temps.hold(value)
	return value
}

func (i *qfnraPurifyInvocation) processIntegerDivision(x, y ast.ASTHandle, quotient *qfnraPurifyEntry) (err error) {
	m := i.owner.manager
	b := qfnraPurifyBuilder{temps: qfnraPurifyTemps{manager: m}}
	defer func() { err = errors.Join(err, b.temps.close()) }()
	mod := b.hold(func() (ast.ASTHandle, error) {
		return m.MkFamilyApp(i.owner.arithmetic, qfnraPurifyMod, nil, []ast.ASTHandle{x, y})
	})
	if b.err != nil {
		return b.err
	}
	remainder, err := i.newEntry(mod)
	if err != nil {
		return err
	}
	q, err := m.RefGet(quotient.fresh)
	if err != nil {
		return err
	}
	r, err := m.RefGet(remainder.fresh)
	if err != nil {
		return err
	}
	zero := b.hold(func() (ast.ASTHandle, error) { return m.ArithmeticIntegerNumeralText("0") })
	minusOne := b.hold(func() (ast.ASTHandle, error) { return m.ArithmeticIntegerNumeralText("-1") })
	yZero := b.hold(func() (ast.ASTHandle, error) { return m.MkEq(y, zero) })
	product := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticMul(q, y) })
	sum := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticAdd(product, r) })
	equation := b.hold(func() (ast.ASTHandle, error) { return m.MkEq(x, sum) })
	first := b.hold(func() (ast.ASTHandle, error) { return m.MkOr(yZero, equation) })
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(first, quotient, remainder); err != nil {
		return err
	}
	positiveR := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticLE(zero, r) })
	second := b.hold(func() (ast.ASTHandle, error) { return m.MkOr(yZero, positiveR) })
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(second, remainder); err != nil {
		return err
	}
	nonpositiveY := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticLE(y, zero) })
	rBelowY := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticLT(r, y) })
	third := b.hold(func() (ast.ASTHandle, error) { return m.MkOr(nonpositiveY, rBelowY) })
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(third, remainder); err != nil {
		return err
	}
	nonnegativeY := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticGE(y, zero) })
	negY := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticMul(minusOne, y) })
	rBelowNegY := b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticLT(r, negY) })
	fourth := b.hold(func() (ast.ASTHandle, error) { return m.MkOr(nonnegativeY, rBelowNegY) })
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(fourth, remainder); err != nil {
		return err
	}
	if !i.owner.complete {
		return nil
	}
	text, _, numeral, err := m.ArithmeticNumeralText(y)
	if err != nil {
		return err
	}
	if numeral && text != "0" {
		return nil
	}
	notZero := b.hold(func() (ast.ASTHandle, error) { return m.MkNot(yZero) })
	divZero := b.hold(func() (ast.ASTHandle, error) {
		return m.MkFamilyApp(i.owner.arithmetic, qfnraPurifyIDiv, nil, []ast.ASTHandle{x, zero})
	})
	qEq := b.hold(func() (ast.ASTHandle, error) { return m.MkEq(q, divZero) })
	qGuard := b.hold(func() (ast.ASTHandle, error) { return m.MkOr(notZero, qEq) })
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(qGuard, quotient); err != nil {
		return err
	}
	modZero := b.hold(func() (ast.ASTHandle, error) {
		return m.MkFamilyApp(i.owner.arithmetic, qfnraPurifyMod, nil, []ast.ASTHandle{x, zero})
	})
	rEq := b.hold(func() (ast.ASTHandle, error) { return m.MkEq(r, modZero) })
	rGuard := b.hold(func() (ast.ASTHandle, error) { return m.MkOr(notZero, rEq) })
	if b.err != nil {
		return b.err
	}
	return i.addConstraint(rGuard, remainder)
}
