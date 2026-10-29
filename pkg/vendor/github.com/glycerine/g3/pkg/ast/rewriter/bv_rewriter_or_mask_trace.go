package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1675-1710; src/util/trace.h:40-46.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (p *BVRewriter) traceORMask(value string, width uint32, arg, result ast.ASTHandle) error {
	tags := p.bvTraceTags()
	if !tags.ShouldTrace("mask_bug") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return nil
	}
	return tags.EmitConditional("mask_bug", "mk_bv_or", "src/ast/rewriter/bv_rewriter.cpp", 1707,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "(assert (distinct (bvor (_ bv"+value+" "+strconv.FormatUint(uint64(width), 10)+")\n"); err != nil {
				return err
			}
			if err := p.writeBVTraceSMT2(w, arg, globals); err != nil {
				return err
			}
			if _, err := io.WriteString(w, ")\n"); err != nil {
				return err
			}
			if err := p.writeBVTraceSMT2(w, result, globals); err != nil {
				return err
			}
			_, err := io.WriteString(w, "))\n")
			return err
		})
}
