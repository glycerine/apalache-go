package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2543-2564; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (p *BVRewriter) traceEqualityRewrite(tag string, line int, lhs, rhs ast.ASTHandle, out ast.ASTRef) error {
	tags := p.bvTraceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return nil
	}
	return tags.EmitConditional(tag, "mk_eq_core", "src/ast/rewriter/bv_rewriter.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if err := p.writeBVTraceSMT2(w, lhs, globals); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n=\n"); err != nil {
				return err
			}
			if err := p.writeBVTraceSMT2(w, rhs, globals); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n----->\n"); err != nil {
				return err
			}
			result, err := p.manager.RefGet(out)
			if err != nil {
				return err
			}
			if err = p.writeBVTraceSMT2(w, result, globals); err != nil {
				return err
			}
			_, err = io.WriteString(w, "\n")
			return err
		})
}
