package core

import "github.com/glycerine/g3/pkg/ast"

type collectStatisticsTraversal struct {
	owner                        *CollectStatisticsTactic
	seenExpr, seenSort, seenDecl map[ast.ASTHandle]bool
	quantifierDepth              uint64
}

func (c *collectStatisticsTraversal) add(name string, amount uint64) { c.owner.stats[name] += amount }

// visit is the all-mark, post-order for_each_expr traversal. Quantifier body,
// patterns and no-patterns are children, in source child order.
func (c *collectStatisticsTraversal) visit(h ast.ASTHandle) error {
	if c.seenExpr[h] {
		return nil
	}
	c.seenExpr[h] = true
	m := c.owner.manager
	k, err := m.GetKind(h)
	if err != nil {
		return err
	}
	switch k {
	case ast.ASTVar:
		return c.variable(h)
	case ast.ASTApp:
		n, e := m.GetNumArgs(h)
		if e != nil {
			return e
		}
		for i := uint32(0); i < n; i++ {
			a, x := m.GetArg(h, i)
			if x != nil {
				return x
			}
			if x = c.visit(a); x != nil {
				return x
			}
		}
		return c.application(h)
	case ast.ASTQuantifier:
		body, e := m.GetQuantifierBody(h)
		if e != nil {
			return e
		}
		if e = c.visit(body); e != nil {
			return e
		}
		n, e := m.GetNumPatterns(h)
		if e != nil {
			return e
		}
		for i := uint32(0); i < n; i++ {
			p, x := m.GetPattern(h, i)
			if x != nil {
				return x
			}
			if x = c.visit(p); x != nil {
				return x
			}
		}
		n, e = m.GetNumNoPatterns(h)
		if e != nil {
			return e
		}
		for i := uint32(0); i < n; i++ {
			p, x := m.GetNoPattern(h, i)
			if x != nil {
				return x
			}
			if x = c.visit(p); x != nil {
				return x
			}
		}
		return c.quantifier(h, body)
	}
	return ast.ErrASTDomain
}

func (c *collectStatisticsTraversal) variable(h ast.ASTHandle) error {
	c.add("bound-variables", 1)
	s, err := c.owner.manager.GetSort(h)
	if err != nil {
		return err
	}
	return c.sort(s)
}

func (c *collectStatisticsTraversal) application(h ast.ASTHandle) error {
	c.add("function-applications", 1)
	d, err := c.owner.manager.GetDecl(h)
	if err != nil {
		return err
	}
	return c.declaration(d)
}

func (c *collectStatisticsTraversal) quantifier(h, body ast.ASTHandle) error {
	c.add("quantifiers", 1)
	n, err := c.owner.manager.GetNumDeclarations(h)
	if err != nil {
		return err
	}
	forall, err := c.owner.manager.IsForall(h)
	if err != nil {
		return err
	}
	if forall {
		c.add("forall-variables", uint64(n))
	} else {
		c.add("exists-variables", uint64(n))
	}
	patterns, err := c.owner.manager.GetNumPatterns(h)
	if err != nil {
		return err
	}
	c.add("patterns", uint64(patterns))
	noPatterns, err := c.owner.manager.GetNumNoPatterns(h)
	if err != nil {
		return err
	}
	c.add("no-patterns", uint64(noPatterns))
	c.quantifierDepth++
	if c.owner.stats["max-quantification-depth"] < c.quantifierDepth {
		c.owner.stats["max-quantification-depth"] = c.quantifierDepth
	}
	// collect_proc directly applies its app overload to the body in addition
	// to the outer for_each_expr traversal.
	if k, e := c.owner.manager.GetKind(body); e != nil {
		return e
	} else if k != ast.ASTApp {
		return ast.ErrASTDomain
	}
	err = c.application(body)
	c.quantifierDepth--
	return err
}
