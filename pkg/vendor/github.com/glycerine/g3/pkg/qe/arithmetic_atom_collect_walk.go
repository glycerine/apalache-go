package qe

import "github.com/glycerine/g3/pkg/ast"

func (c *arithmeticAtomCollector) walk(expression ast.ASTHandle, positive bool) error {
	v := arithmeticAtomVisit{expression, positive}
	if c.seen[v] {
		return nil
	}
	c.seen[v] = true
	kind, boolean, e := c.boolean(expression)
	if e != nil {
		return e
	}
	if !boolean {
		return c.leaf(expression, positive)
	}
	count, e := c.manager.GetNumArgs(expression)
	if e != nil {
		return e
	}
	if kind == ast.ASTOpNot {
		if count != 1 {
			return ast.ErrASTDomain
		}
		a, e := c.manager.GetArg(expression, 0)
		if e != nil {
			return e
		}
		return c.walk(a, !positive)
	}
	for i := uint32(0); i < count; i++ {
		a, e := c.manager.GetArg(expression, i)
		if e != nil {
			return e
		}
		if e = c.walk(a, positive); e != nil {
			return e
		}
	}
	return nil
}
