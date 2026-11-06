package core

import "github.com/glycerine/g3/pkg/ast"

func domOp(m *ast.ASTManager, h ast.ASTHandle, op int32) (bool, error) {
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return false, e
	}
	f, e := m.GetFamilyID(h)
	if e != nil || f != m.BasicFamilyID() {
		return false, e
	}
	v, e := m.GetDeclKind(h)
	return v == op, e
}

func (t *DomSimplifyTactic) simplify(h ast.ASTHandle, c *domContext, depth uint32) (ast.ASTHandle, error) {
	original := h
	h = t.domCached(h)
	m := t.manager
	if depth >= t.maxDepth {
		r := c.find(h)
		if err := t.cacheDom(original, r); err != nil {
			return h, err
		}
		return r, nil
	}
	k, e := m.GetKind(h)
	if e != nil {
		return h, e
	}
	r := h
	if k == ast.ASTApp {
		if yes, x := domOp(m, h, ast.ASTOpIte); x != nil {
			return h, x
		} else if yes {
			r, e = t.simplifyITE(h, c, depth)
		} else if yes, x := domOp(m, h, ast.ASTOpAnd); x != nil {
			return h, x
		} else if yes {
			r, e = t.simplifyAndOr(h, c, depth, true)
		} else if yes, x := domOp(m, h, ast.ASTOpOr); x != nil {
			return h, x
		} else if yes {
			r, e = t.simplifyAndOr(h, c, depth, false)
		} else {
			// C++ simplify_rec only recursively visits immediate dominator-tree
			// children. Other arguments receive a direct cached substitution.
			for _, child := range t.dominators.tree[h] {
				if child != h {
					if _, e = t.simplify(child, c, depth+1); e != nil {
						return h, e
					}
				}
			}
			n, x := m.GetNumArgs(h)
			if x != nil {
				return h, x
			}
			args := make([]ast.ASTHandle, n)
			for i := uint32(0); i < n; i++ {
				a, x := m.GetArg(h, i)
				if x != nil {
					return h, x
				}
				args[i] = c.find(t.domCached(a))
			}
			if yes, arg, x := m.IsNot(h); x != nil {
				return h, x
			} else if yes {
				truth, x := m.MkTrue()
				if x != nil {
					return h, x
				}
				falsity, x := m.MkFalse()
				if x != nil {
					return h, x
				}
				switch args[0] {
				case truth:
					r = falsity
				case falsity:
					r = truth
				default:
					if args[0] == arg {
						r = h
					} else {
						r, x = m.MkNot(args[0])
					}
				}
				if x != nil {
					return h, x
				}
			} else {
				if yes, x := domOp(m, h, ast.ASTOpEq); x != nil {
					return h, x
				} else if !yes {
					yes, x = domOp(m, h, ast.ASTOpIff)
					if x != nil {
						return h, x
					}
				}
				if yes && len(args) == 2 && args[0] == args[1] {
					r, e = m.MkTrue()
				} else {
					d, x := m.GetDecl(h)
					if x != nil {
						return h, x
					}
					r, e = m.MkApp(d, args)
				}
			}
		}
	}
	if e != nil {
		return h, e
	}
	r = c.find(r)
	if e = t.cacheDom(original, r); e != nil {
		return h, e
	}
	return r, nil
}

func (t *DomSimplifyTactic) simplifyAndOr(h ast.ASTHandle, c *domContext, depth uint32, and bool) (ast.ASTHandle, error) {
	m := t.manager
	n, e := m.GetNumArgs(h)
	if e != nil {
		return h, e
	}
	level := c.level()
	args := make([]ast.ASTHandle, n)
	originals := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		originals[i], e = m.GetArg(h, i)
		if e != nil {
			return h, e
		}
	}
	order := make([]uint32, n)
	for i := range order {
		if t.forward {
			order[i] = uint32(i)
		} else {
			order[i] = n - 1 - uint32(i)
		}
	}
	for _, i := range order {
		a := originals[i]
		for _, child := range t.dominators.tree[a] {
			exclusive, x := t.domExclusive(child, a, originals)
			if x != nil {
				c.pop(level)
				return h, x
			}
			if exclusive {
				if _, x = t.simplify(child, c, depth+1); x != nil {
					c.pop(level)
					return h, x
				}
			}
		}
		r := c.find(t.domCached(a))
		args[i] = r
		if x := t.assertContext(c, r, !and); x != nil {
			c.pop(level)
			return h, x
		}
	}
	c.pop(level)
	if and {
		return m.MkAndArgs(args)
	}
	return m.MkOrArgs(args)
}

func (t *DomSimplifyTactic) simplifyITE(h ast.ASTHandle, c *domContext, depth uint32) (ast.ASTHandle, error) {
	m := t.manager
	condition, e := m.GetArg(h, 0)
	if e != nil {
		return h, e
	}
	thenBranch, e := m.GetArg(h, 1)
	if e != nil {
		return h, e
	}
	elseBranch, e := m.GetArg(h, 2)
	if e != nil {
		return h, e
	}
	condition = c.find(t.domCached(condition))
	truth, e := m.MkTrue()
	if e != nil {
		return h, e
	}
	falsity, e := m.MkFalse()
	if e != nil {
		return h, e
	}
	if condition == truth {
		return c.find(t.domCached(thenBranch)), nil
	}
	if condition == falsity {
		return c.find(t.domCached(elseBranch)), nil
	}
	level := c.level()
	if e = t.assertContext(c, condition, false); e != nil {
		return h, e
	}
	for _, child := range t.dominators.tree[h] {
		a, x := t.domSubexpr(child, thenBranch, make(map[ast.ASTHandle]bool))
		if x != nil {
			c.pop(level)
			return h, x
		}
		b, x := t.domSubexpr(child, elseBranch, make(map[ast.ASTHandle]bool))
		if x != nil {
			c.pop(level)
			return h, x
		}
		if a && !b {
			if _, x = t.simplify(child, c, depth+1); x != nil {
				c.pop(level)
				return h, x
			}
		}
	}
	c.pop(level)
	yes := c.find(t.domCached(thenBranch))
	if e = t.assertContext(c, condition, true); e != nil {
		return h, e
	}
	for _, child := range t.dominators.tree[h] {
		a, x := t.domSubexpr(child, elseBranch, make(map[ast.ASTHandle]bool))
		if x != nil {
			c.pop(level)
			return h, x
		}
		b, x := t.domSubexpr(child, thenBranch, make(map[ast.ASTHandle]bool))
		if x != nil {
			c.pop(level)
			return h, x
		}
		if a && !b {
			if _, x = t.simplify(child, c, depth+1); x != nil {
				c.pop(level)
				return h, x
			}
		}
	}
	c.pop(level)
	no := c.find(t.domCached(elseBranch))
	if yes == no {
		return yes, nil
	}
	if yes == thenBranch && no == elseBranch {
		return h, nil
	}
	return m.MkIte(condition, yes, no)
}
