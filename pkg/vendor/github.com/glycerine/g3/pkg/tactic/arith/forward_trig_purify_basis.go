package arith

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
)

func (i *forwardTrigInvocation) basis(theta ast.ASTHandle) (_ *forwardTrigPair, ok bool, err error) {
	yes, e := i.isUninterpretedConst(theta)
	if e != nil || !yes || i.unsafe[theta] {
		return nil, false, e
	}
	if p := i.pairs[theta]; p != nil {
		return p, true, nil
	}
	m := i.owner.manager
	sort, e := m.GetSort(theta)
	if e != nil {
		return nil, false, e
	}
	s, e := m.MkFreshConstNullable(nil, sort)
	if e != nil {
		return nil, false, e
	}
	sr, e := m.NewRef(s)
	if e != nil {
		return nil, false, e
	}
	c, e := m.MkFreshConstNullable(nil, sort)
	if e != nil {
		return nil, false, errors.Join(e, m.DelRef(&sr))
	}
	cr, e := m.NewRef(c)
	if e != nil {
		return nil, false, errors.Join(e, m.DelRef(&sr))
	}
	thetaRef, e := m.NewRef(theta)
	if e != nil {
		return nil, false, errors.Join(e, m.DelRef(&cr), m.DelRef(&sr))
	}
	p := &forwardTrigPair{theta: theta, thetaRef: thetaRef, sin: sr, cos: cr}
	i.pairs[theta] = p
	i.pairOrder = append(i.pairOrder, p)
	return p, true, nil
}
