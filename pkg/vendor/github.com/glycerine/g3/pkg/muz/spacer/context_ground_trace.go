package spacer

// Source: src/muz/spacer/spacer_context.cpp:2560 get_ground_sat_answer.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *Context) traceGroundCounterexample(facts []ast.ASTRef) error {
	tags := c.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("spacer", "get_ground_sat_answer", "src/muz/spacer/spacer_context.cpp", 2560,
		func() bool { return true }, func(out io.Writer) error {
			printer, err := ast.NewSimpleASTPrinterContext(c.manager, out, out)
			if err != nil {
				return err
			}
			_, _ = io.WriteString(out, "ground cex\n")
			for _, ref := range facts {
				fact, err := c.manager.RefGet(ref)
				if err != nil {
					return err
				}
				if err = printer.DisplayExpr(out, fact, 0); err != nil && !tags.IsStreamFailure(err) {
					return err
				}
				_, _ = io.WriteString(out, "\n")
			}
			return nil
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
