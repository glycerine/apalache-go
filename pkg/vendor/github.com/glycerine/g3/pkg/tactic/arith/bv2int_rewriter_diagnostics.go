package arith

// Source: src/tactic/arith/bv2int_rewriter.cpp assertions and TRACE236/251.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func bv2IntSourceAssert(line int32, condition string, ok bool) error {
	enabled, err := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
	if err != nil || !enabled || ok {
		return err
	}
	return diagnostic.DebugAssertionFailure(diagnostic.BorrowProcessDebugContext(), "src/tactic/arith/bv2int_rewriter.cpp", line, condition)
}
func (c *bv2IntCall) assertBV(h ast.ASTHandle, line int32, condition string) {
	if c.err != nil {
		return
	}
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled {
		c.err = err
		return
	}
	sort, err := c.r.manager.GetSort(h)
	if err != nil {
		c.err = err
		return
	}
	yes, err := c.r.manager.IsBitVectorSort(sort)
	if err != nil {
		c.err = err
		return
	}
	if !yes {
		c.err = diagnostic.DebugAssertionFailure(d, "src/tactic/arith/bv2int_rewriter.cpp", line, condition)
	}
}
func (r *BV2IntRewriter) traceMod(h ast.ASTHandle, line int) error {
	tags := r.context.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("bv2int_rewriter", "mk_mod", "src/tactic/arith/bv2int_rewriter.cpp", line, func() bool { return true }, func(out io.Writer) error {
		if err := r.context.writeExpr(out, h); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
