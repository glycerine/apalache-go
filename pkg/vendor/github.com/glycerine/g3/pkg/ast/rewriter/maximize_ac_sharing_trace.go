package rewriter

// Source: src/ast/rewriter/maximize_ac_sharing.cpp TRACE sites; src/util/trace.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (c *MaximizeBVSharingConfiguration) sharingTraceTags() *diagnostic.TraceTags {
	if c.trace != nil {
		return c.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

// BindTrace borrows a stream for the retained source diagnostics.
func (c *MaximizeBVSharingConfiguration) BindTrace(tags *diagnostic.TraceTags) error {
	if err := c.open(); err != nil {
		return err
	}
	c.trace = tags
	return nil
}

func (c *MaximizeBVSharingConfiguration) emitSharingTrace(function string, line int, body func(io.Writer, *uparams.GlobalParamsService) error) error {
	tags := c.sharingTraceTags()
	if !tags.ShouldTrace("ac_sharing_detail") {
		return nil
	}
	g := uparams.BorrowProcessGlobalParamsService()
	if g == nil || g.Context == nil {
		return nil
	}
	return emitRewriterSourceTrace(tags, "ac_sharing_detail", function, "src/ast/rewriter/maximize_ac_sharing.cpp", line,
		func(w io.Writer) error { return body(w, g) })
}

func (c *MaximizeBVSharingConfiguration) writeSharingAST(w io.Writer, h ast.ASTHandle, g *uparams.GlobalParamsService) error {
	return writeBVTraceSMT2(c.manager, w, h, g)
}
