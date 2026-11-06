package spacer

// Source: spacer_context.cpp pred_transformer::add_reach_fact TRACE.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *PredicateTransformer) traceAddReachFact(formula ast.ASTHandle, initial bool) error {
	tags := p.owner.contexts[0].config.ModelEvaluator.Trace
	if tags == nil {
		tags = p.manager.BorrowSourceTraceTags()
	}
	err := tags.EmitConditional("spacer", "add_reach_fact", "src/muz/spacer/spacer_context.cpp", 410,
		func() bool { return true }, func(out io.Writer) error {
			_, _ = io.WriteString(out, "add_reach_fact: ")
			_, _ = io.WriteString(out, p.name)
			_, _ = io.WriteString(out, " ")
			if initial {
				_, _ = io.WriteString(out, "INIT ")
			}
			printer, err := ast.NewSimpleASTPrinterContext(p.manager, out, out)
			if err != nil {
				return err
			}
			if err = printer.DisplayExpr(out, formula, 0); err != nil && !tags.IsStreamFailure(err) {
				return err
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
