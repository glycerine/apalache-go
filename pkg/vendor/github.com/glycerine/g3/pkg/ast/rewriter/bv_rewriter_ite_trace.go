package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2657-2660; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (p *BVRewriter) traceITECore(condition, then, otherwise ast.ASTHandle) error {
	tags := p.bvTraceTags()
	if !tags.ShouldTrace("bv_ite") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return nil
	}
	return tags.EmitConditional("bv_ite", "mk_ite_core", "src/ast/rewriter/bv_rewriter.cpp", 2658,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "mk_ite_core:\n"); err != nil {
				return err
			}
			if err := p.writeBVTraceSMT2(w, condition, globals); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "?\n"); err != nil {
				return err
			}
			if err := p.writeBVTraceSMT2(w, then, globals); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n:"); err != nil {
				return err
			}
			if err := p.writeBVTraceSMT2(w, otherwise, globals); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
}
