package rewriter

// Source: src/ast/rewriter/push_app_ite.cpp TRACE sites; src/util/trace.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *PushAppITEConfiguration) pushITETraceTags() *diagnostic.TraceTags {
	if c.trace != nil {
		return c.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

// BindTrace borrows a stream for the retained push_app_ite diagnostics.
func (c *PushAppITEConfiguration) BindTrace(tags *diagnostic.TraceTags) error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if _, err := c.manager.ProofsEnabled(); err != nil {
		return err
	}
	c.trace = tags
	return nil
}

// BindTrace forwards trace ownership through the public rewriter wrapper.
func (r *PushAppITE) BindTrace(tags *diagnostic.TraceTags) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.config.BindTrace(tags)
}

func (c *PushAppITEConfiguration) emitPushITETrace(function string, line int, body func(io.Writer, *uparams.GlobalParamsService) error) error {
	tags := c.pushITETraceTags()
	if !tags.ShouldTrace("push_app_ite") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return nil
	}
	return emitRewriterSourceTrace(tags, "push_app_ite", function, "src/ast/rewriter/push_app_ite.cpp", line,
		func(w io.Writer) error { return body(w, globals) })
}
