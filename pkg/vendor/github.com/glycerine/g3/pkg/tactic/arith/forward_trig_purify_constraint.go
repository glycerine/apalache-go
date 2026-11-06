package arith

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (i *forwardTrigInvocation) unitCircle(pair *forwardTrigPair, entry *forwardTrigEntry) (err error) {
	return forwardTrigUnitCircle(i.owner.manager, pair, func(h ast.ASTHandle) error { return i.addConstraint(h, entry) })
}

func forwardTrigUnitCircle(m *ast.ASTManager, pair *forwardTrigPair, add func(ast.ASTHandle) error) (err error) {
	t := forwardTrigTemps{manager: m}
	defer func() { err = errors.Join(err, t.close()) }()
	x, e := m.RefGet(pair.sin)
	if e != nil {
		return e
	}
	y, e := m.RefGet(pair.cos)
	if e != nil {
		return e
	}
	xx, e := m.MkArithmeticMul(x, x)
	if e != nil {
		return e
	}
	if _, e = t.hold(xx); e != nil {
		return e
	}
	yy, e := m.MkArithmeticMul(y, y)
	if e != nil {
		return e
	}
	if _, e = t.hold(yy); e != nil {
		return e
	}
	sum, e := m.MkArithmeticAdd(xx, yy)
	if e != nil {
		return e
	}
	if _, e = t.hold(sum); e != nil {
		return e
	}
	one, e := m.ArithmeticRealNumeralText("1")
	if e != nil {
		return e
	}
	if _, e = t.hold(one); e != nil {
		return e
	}
	eq, e := m.MkEq(one, sum)
	if e != nil {
		return e
	}
	return add(eq)
}
