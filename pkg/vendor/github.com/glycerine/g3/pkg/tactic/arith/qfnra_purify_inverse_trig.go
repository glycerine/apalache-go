package arith

// Source: src/tactic/arith/purify_arith_tactic.cpp process_asin/acos/atan.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (i *qfnraPurifyInvocation) processInverseTrig(op int32, x, fresh ast.ASTHandle, entry *qfnraPurifyEntry) (err error) {
	m := i.owner.manager
	b := qfnraPurifyBuilder{temps: qfnraPurifyTemps{manager: m}}
	defer func() { err = errors.Join(err, b.temps.close()) }()
	real := func(value string) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.ArithmeticRealNumeralText(value) })
	}
	app := func(kind int32, args ...ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkFamilyApp(i.owner.arithmetic, kind, nil, args) })
	}
	eq := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkEq(a, c) })
	}
	and := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkAnd(a, c) })
	}
	or := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkOr(a, c) })
	}
	not := func(a ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkNot(a) })
	}
	ge := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticGE(a, c) })
	}
	le := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticLE(a, c) })
	}
	gt := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticGT(a, c) })
	}
	lt := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticLT(a, c) })
	}
	mul := func(a, c ast.ASTHandle) ast.ASTHandle {
		return b.hold(func() (ast.ASTHandle, error) { return m.MkArithmeticMul(a, c) })
	}
	pi := app(forwardTrigPi)
	if op == qfnraPurifyATan {
		halfPi := mul(real("1/2"), pi)
		minusHalfPi := mul(real("-1/2"), pi)
		constraint := and(eq(x, app(21, fresh)), and(gt(fresh, minusHalfPi), lt(fresh, halfPi)))
		if b.err != nil {
			return b.err
		}
		return i.addConstraint(constraint, entry)
	}
	one, minusOne := real("1"), real("-1")
	lowerX, upperX := ge(x, minusOne), le(x, one)
	outside := or(not(lowerX), not(upperX))
	var forward, low, high ast.ASTHandle
	var undefinedKind int32
	if op == qfnraPurifyASin {
		forward = app(forwardTrigSin, fresh)
		low, high = mul(real("-1/2"), pi), mul(real("1/2"), pi)
		undefinedKind = 34
	} else {
		forward = app(forwardTrigCos, fresh)
		low, high = real("0"), pi
		undefinedKind = 35
	}
	inside := and(eq(x, forward), and(ge(fresh, low), le(fresh, high)))
	constraint := or(outside, inside)
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(constraint, entry); err != nil {
		return err
	}
	if !i.owner.complete {
		return nil
	}
	undefined := app(undefinedKind, x)
	undefinedEq := eq(fresh, undefined)
	lowerGuard := or(lowerX, undefinedEq)
	upperGuard := or(upperX, undefinedEq)
	if b.err != nil {
		return b.err
	}
	if err = i.addConstraint(lowerGuard, entry); err != nil {
		return err
	}
	return i.addConstraint(upperGuard, entry)
}
