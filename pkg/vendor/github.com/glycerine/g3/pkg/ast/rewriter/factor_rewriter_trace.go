package rewriter

// Source: src/ast/rewriter/factor_rewriter.cpp TRACE sites; src/util/trace.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (r *FactorRewriter) factorTraceTags() *diagnostic.TraceTags {
	if r.trace != nil {
		return r.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

// BindTrace borrows a stream for the retained factor_rewriter diagnostics.
func (r *FactorRewriter) BindTrace(tags *diagnostic.TraceTags) error {
	if err := r.open(); err != nil {
		return err
	}
	r.trace = tags
	return nil
}

func (r *FactorRewriter) emitFactorTrace(function string, line int, body func(io.Writer, *uparams.GlobalParamsService) error) error {
	tags := r.factorTraceTags()
	if !tags.ShouldTrace("factor_rewriter") {
		return nil
	}
	g := uparams.BorrowProcessGlobalParamsService()
	if g == nil || g.Context == nil {
		return nil
	}
	return emitRewriterSourceTrace(tags, "factor_rewriter", function, "src/ast/rewriter/factor_rewriter.cpp", line,
		func(w io.Writer) error { return body(w, g) })
}

func (r *FactorRewriter) writeFactorAST(w io.Writer, h ast.ASTHandle, g *uparams.GlobalParamsService) error {
	return writeBVTraceSMT2(r.manager, w, h, g)
}
