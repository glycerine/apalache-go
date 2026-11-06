package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:601-610,623-632,655-664.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// publishFold mirrors assignments to the caller's expr_ref. Earlier completed
// prefixes remain owned by that reference when a later binary operation fails.
func (c *bv2RealCall) publishFold(h ast.ASTHandle) {
	if c.output == (ast.ASTRef{}) || c.err != nil {
		return
	}
	c.err = c.u.manager.SetRef(c.output, h)
	if c.err == nil {
		c.outputAssigned = true
	}
}

func (c *bv2RealCall) fold(op int32, args []ast.ASTHandle) (ast.ASTHandle, rewriter.RewriteStatus) {
	if len(args) == 0 {
		c.err = ast.ErrASTDomain
		return ast.ASTHandle{}, rewriter.RewriteFailed
	}
	mark := c.checkpoint()
	h := args[0]
	c.publishFold(h)
	if c.err != nil {
		return ast.ASTHandle{}, rewriter.RewriteFailed
	}
	for _, arg := range args[1:] {
		next, ok := c.binary(op, h, arg)
		if c.err != nil {
			return ast.ASTHandle{}, rewriter.RewriteFailed
		}
		if ok {
			h = next
			c.publishFold(h)
		}
		c.endExpressionScope(mark, &h)
		if c.err != nil {
			return ast.ASTHandle{}, rewriter.RewriteFailed
		}
		if !ok {
			return h, rewriter.RewriteFailed
		}
	}
	return h, rewriter.RewriteDone
}
