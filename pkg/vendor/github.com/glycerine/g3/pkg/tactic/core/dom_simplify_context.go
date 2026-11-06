package core

import "github.com/glycerine/g3/pkg/ast"

type domContext struct {
	scopes []map[ast.ASTHandle]ast.ASTHandle
}

func (c *domContext) level() int { return len(c.scopes) }
func (c *domContext) push()      { c.scopes = append(c.scopes, make(map[ast.ASTHandle]ast.ASTHandle)) }
func (c *domContext) pop(level int) {
	if level < 0 {
		level = 0
	}
	c.scopes = c.scopes[:level]
}
func (c *domContext) find(h ast.ASTHandle) ast.ASTHandle {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if v, ok := c.scopes[i][h]; ok {
			return v
		}
	}
	return h
}
func (c *domContext) insert(k, v ast.ASTHandle) {
	if len(c.scopes) == 0 {
		c.push()
	}
	c.scopes[len(c.scopes)-1][k] = v
}

func (t *DomSimplifyTactic) assertContext(c *domContext, h ast.ASTHandle, sign bool) error {
	c.push()
	m := t.manager
	if sign {
		if yes, arg, e := m.IsNot(h); e != nil {
			return e
		} else if yes {
			h = arg
		} else {
			n, e := m.MkNot(h)
			if e != nil {
				return e
			}
			h = n
		}
	}
	if yes, arg, e := m.IsNot(h); e != nil {
		return e
	} else if yes {
		f, e := m.MkFalse()
		if e != nil {
			return e
		}
		c.insert(arg, f)
		return nil
	}
	isEquality, e := domOp(m, h, ast.ASTOpEq)
	if e != nil {
		return e
	}
	if !isEquality {
		isEquality, e = domOp(m, h, ast.ASTOpIff)
		if e != nil {
			return e
		}
	}
	if isEquality {
		lhs, e := m.GetArg(h, 0)
		if e != nil {
			return e
		}
		rhs, e := m.GetArg(h, 1)
		if e != nil {
			return e
		}
		lg, e := m.IsGround(lhs)
		if e != nil {
			return e
		}
		rg, e := m.IsGround(rhs)
		if e != nil {
			return e
		}
		if lg && rg {
			greater, e := t.greater(lhs, rhs)
			if e != nil {
				return e
			}
			if greater {
				c.insert(lhs, rhs)
			} else {
				greater, e = t.greater(rhs, lhs)
				if e != nil {
					return e
				}
				if greater {
					c.insert(rhs, lhs)
				}
			}
			return nil
		}
	}
	truth, e := m.MkTrue()
	if e != nil {
		return e
	}
	c.insert(h, truth)
	return nil
}

func (t *DomSimplifyTactic) greater(a, b ast.ASTHandle) (bool, error) {
	if a == b {
		return false, nil
	}
	value, e := t.manager.IsValue(b)
	if e != nil {
		return false, e
	}
	if value {
		return true, nil
	}
	ad, e := t.manager.GetDepth(a)
	if e != nil {
		return false, e
	}
	bd, e := t.manager.GetDepth(b)
	if e != nil {
		return false, e
	}
	if ad != bd {
		return ad > bd, nil
	}
	ak, e := t.manager.GetKind(a)
	if e != nil {
		return false, e
	}
	bk, e := t.manager.GetKind(b)
	if e != nil {
		return false, e
	}
	if ak != ast.ASTApp || bk != ast.ASTApp {
		return false, nil
	}
	ax, e := t.manager.GetDecl(a)
	if e != nil {
		return false, e
	}
	bx, e := t.manager.GetDecl(b)
	if e != nil {
		return false, e
	}
	aid, e := t.manager.GetASTID(ax)
	if e != nil {
		return false, e
	}
	bid, e := t.manager.GetASTID(bx)
	if e != nil {
		return false, e
	}
	if aid != bid {
		return aid > bid, nil
	}
	an, e := t.manager.GetNumArgs(a)
	if e != nil {
		return false, e
	}
	bn, e := t.manager.GetNumArgs(b)
	if e != nil {
		return false, e
	}
	if an != bn {
		return an > bn, nil
	}
	for i := uint32(0); i < an; i++ {
		aa, e := t.manager.GetArg(a, i)
		if e != nil {
			return false, e
		}
		bb, e := t.manager.GetArg(b, i)
		if e != nil {
			return false, e
		}
		if aa != bb {
			return t.greater(aa, bb)
		}
	}
	return false, nil
}
