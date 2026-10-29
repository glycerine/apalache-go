package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:690.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// assertRewrittenFixedPoint performs the source's second rewrite only while
// source assertions are active. The second result is a temporary owned ref.
func (r *formulaProcessor) assertRewrittenFixedPoint(rewritten ast.ASTHandle) error {
	enabled, e := forwardAssertionEnabled()
	if e != nil || !enabled {
		return e
	}
	second, e := r.loop.rewrite(r.index, r.bool, rewritten)
	if e != nil {
		return e
	}
	h, getErr := r.manager.RefGet(second)
	closeErr := r.manager.DelRef(&second)
	if e := errors.Join(getErr, closeErr); e != nil {
		return e
	}
	if h != rewritten {
		return forwardAssertionFailure(690, "rewrite(np)==np")
	}
	return nil
}
