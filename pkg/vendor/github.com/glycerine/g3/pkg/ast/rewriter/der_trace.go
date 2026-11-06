package rewriter

// Source: src/ast/rewriter/der.cpp TRACE sites; src/util/trace.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (d *DER) derTraceTags() *diagnostic.TraceTags {
	if d.trace != nil {
		return d.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

// BindTrace borrows a stream for retained DER diagnostics.
func (d *DER) BindTrace(tags *diagnostic.TraceTags) error {
	if err := d.open(); err != nil {
		return err
	}
	d.trace = tags
	return nil
}

func (c *DERConfiguration) BindTrace(tags *diagnostic.TraceTags) error {
	if c == nil || c.self != c || c.closed || c.core == nil {
		return ast.ErrASTHandle
	}
	return c.core.BindTrace(tags)
}

func (r *DERRewriter) BindTrace(tags *diagnostic.TraceTags) error {
	if r == nil || r.self != r || r.closed || r.config == nil {
		return ast.ErrASTHandle
	}
	return r.config.BindTrace(tags)
}

func (d *DER) emitDERTrace(tag, function string, line int, body func(io.Writer, *uparams.GlobalParamsService) error) error {
	tags := d.derTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return nil
	}
	return emitRewriterSourceTrace(tags, tag, function, "src/ast/rewriter/der.cpp", line,
		func(w io.Writer) error { return body(w, globals) })
}
