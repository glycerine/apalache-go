package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:770-780; src/util/trace.h:40-46.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (p *BVRewriter) traceExtractPropagation(arg, result ast.ASTHandle, high, low, removed uint32) error {
	tags := p.bvTraceTags()
	if !tags.ShouldTrace("extract_prop") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return nil
	}
	return tags.EmitConditional("extract_prop", "mk_extract_core", "src/ast/rewriter/bv_rewriter.cpp", 776,
		func() bool { return true }, func(w io.Writer) error {
			if err := p.writeBVTraceSMT2(w, arg, globals); err != nil {
				return err
			}
			text := "\n[" + strconv.FormatUint(uint64(high), 10) + "," + strconv.FormatUint(uint64(low), 10) + "]\n" + strconv.FormatUint(uint64(removed), 10) + "---->\n"
			if _, err := io.WriteString(w, text); err != nil {
				return err
			}
			if err := p.writeBVTraceSMT2(w, result, globals); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
}
