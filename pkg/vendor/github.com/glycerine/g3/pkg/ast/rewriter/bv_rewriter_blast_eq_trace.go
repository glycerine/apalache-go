package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2281-2285; src/util/trace.h:40-46.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// BindTrace borrows the source trace stream for the directly callable polynomial owner.
func (p *BVPolynomialRewriter) BindTrace(tags *diagnostic.TraceTags) error {
	if err := p.open(); err != nil {
		return err
	}
	p.trace = tags
	return nil
}

func (p *BVPolynomialRewriter) traceBlastEqualityValue(width uint32, lhs ast.ASTHandle) error {
	tags := p.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("blast_eq_value") {
		return nil
	}
	globals := uparams.BorrowProcessGlobalParamsService()
	if globals == nil || globals.Context == nil {
		return nil
	}
	return tags.EmitConditional("blast_eq_value", "mk_blast_eq_value", "src/ast/rewriter/bv_rewriter.cpp", 2283,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "sz: "+strconv.FormatUint(uint64(width), 10)+"\n"); err != nil {
				return err
			}
			if err := writeBVTraceSMT2(p.manager, w, lhs, globals); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
}
