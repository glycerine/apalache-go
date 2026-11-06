package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2781; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *BVRewriter) traceSignedMultiplyUnderflow(n *ast.BitVectorMultiplyBoundNumerals) error {
	tags := p.bvTraceTags()
	if !tags.ShouldTrace("bv_rewriter_bvsmul_no_underflow") {
		return nil
	}
	return tags.EmitConditional("bv_rewriter_bvsmul_no_underflow", "mk_bvsmul_no_underflow", "src/ast/rewriter/bv_rewriter.cpp", 2781,
		func() bool { return true }, func(w io.Writer) error {
			values, err := n.SignedUnderflowTraceValues()
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, "a0:"+values[0]+" a1:"+values[1]+" mr:"+values[2]+" neg_lim:"+values[3]+"\n")
			return err
		})
}
