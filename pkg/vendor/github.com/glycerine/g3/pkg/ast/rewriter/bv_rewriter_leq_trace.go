package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:509-526; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (p *BVRewriter) traceLEExtra(line int, signed bool, a, b ast.ASTHandle, out ast.ASTRef) error {
	tags := p.bvTraceTags()
	if !tags.ShouldTrace("le_extra") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return nil
	}
	return tags.EmitConditional("le_extra", "mk_bv_le_core", "src/ast/rewriter/bv_rewriter.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			label := "bv_ule\n"
			if signed {
				label = "bv_sle\n"
			}
			if _, err := io.WriteString(w, label); err != nil {
				return err
			}
			for _, h := range [...]ast.ASTHandle{a, b} {
				if err := p.writeBVTraceSMT2(w, h, globals); err != nil {
					return err
				}
				if _, err := io.WriteString(w, "\n"); err != nil {
					return err
				}
			}
			if _, err := io.WriteString(w, "--->\n"); err != nil {
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
