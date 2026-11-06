package arith

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (i *forwardTrigInvocation) publishModel(out *tactic.TacticResult) (err error) {
	if !i.models || len(i.pairOrder) == 0 {
		return nil
	}
	m := i.owner.manager
	filter, e := tactic.NewFilterModel(m)
	if e != nil {
		return e
	}
	if e = filter.IncRef(); e != nil {
		return errors.Join(e, filter.Close())
	}
	defer func() { err = errors.Join(err, filter.DecRef()) }()
	extension, e := tactic.NewExtensionModel(m, i.owner.services)
	if e != nil {
		return e
	}
	if e = extension.IncRef(); e != nil {
		return errors.Join(e, extension.Close())
	}
	defer func() { err = errors.Join(err, extension.DecRef()) }()
	for _, pair := range i.pairOrder {
		x, e := m.RefGet(pair.sin)
		if e != nil {
			return e
		}
		y, e := m.RefGet(pair.cos)
		if e != nil {
			return e
		}
		for _, fresh := range []ast.ASTHandle{x, y} {
			d, e := m.GetDecl(fresh)
			if e != nil {
				return e
			}
			if e = filter.Insert(d); e != nil {
				return e
			}
		}
		definition, e := i.thetaDefinition(x, y)
		if e != nil {
			return e
		}
		d, e := m.GetDecl(pair.theta)
		if e != nil {
			return e
		}
		if e = extension.Insert(d, definition); e != nil {
			return e
		}
	}
	combined, e := tactic.ConcatModels(m.Budget(), filter, extension)
	if e != nil {
		return e
	}
	if e = out.SetModelConverter(combined); e != nil {
		return errors.Join(e, combined.Close())
	}
	return nil
}
func (i *forwardTrigInvocation) thetaDefinition(x, y ast.ASTHandle) (_ ast.ASTHandle, err error) {
	m := i.owner.manager
	t := forwardTrigTemps{manager: m}
	defer func() { err = errors.Join(err, t.close()) }()
	zero, e := m.ArithmeticRealNumeralText("0")
	if e != nil {
		return ast.ASTHandle{}, e
	}
	nonnegative, e := m.MkArithmeticGE(x, zero)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	t.hold(nonnegative)
	acosY, e := m.MkFamilyApp(i.owner.arithmetic, forwardTrigACos, nil, []ast.ASTHandle{y})
	if e != nil {
		return ast.ASTHandle{}, e
	}
	t.hold(acosY)
	negativeY, e := m.MkArithmeticNeg(y)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	t.hold(negativeY)
	acosNegative, e := m.MkFamilyApp(i.owner.arithmetic, forwardTrigACos, nil, []ast.ASTHandle{negativeY})
	if e != nil {
		return ast.ASTHandle{}, e
	}
	t.hold(acosNegative)
	pi, e := m.MkFamilyApp(i.owner.arithmetic, forwardTrigPi, nil, nil)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	t.hold(pi)
	sum, e := m.MkArithmeticAdd(acosNegative, pi)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	t.hold(sum)
	return m.MkIte(nonnegative, acosY, sum)
}
