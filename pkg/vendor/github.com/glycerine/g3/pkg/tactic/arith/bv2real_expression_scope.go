package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp local expr_ref scopes in
// signed numeral construction, extension, arithmetic and real decoding.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// endExpressionScope moves the returned expression's existing reference into
// the caller scope before destroying local owners. A borrowed input already
// has its caller's owner and needs no new reference. Failed cleanup stays on
// the call so the utility can finish it before accepting more work.
func (c *bv2RealCall) endExpressionScope(mark bv2RealCallCheckpoint, result *ast.ASTHandle) {
	var escaped ast.ASTRef
	if c.err == nil && !result.IsNull() {
		for i := len(c.refs) - 1; i >= mark.refs; i-- {
			h, err := c.u.manager.RefGet(c.refs[i])
			if err != nil {
				c.err = err
				break
			}
			if h == *result {
				escaped = c.refs[i]
				copy(c.refs[i:], c.refs[i+1:])
				c.refs[len(c.refs)-1] = ast.ASTRef{}
				c.refs = c.refs[:len(c.refs)-1]
				break
			}
		}
	}
	c.err = errors.Join(c.err, c.releaseTo(mark))
	if escaped != (ast.ASTRef{}) {
		c.refs = append(c.refs, escaped)
	}
	if c.err != nil {
		*result = ast.ASTHandle{}
	}
}
