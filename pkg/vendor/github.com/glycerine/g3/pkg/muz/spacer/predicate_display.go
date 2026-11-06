package spacer

// Source: src/muz/spacer/spacer_context.cpp pred_transformer::display and the
// context::init_rules TRACE immediately after transformer initialization.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *PredicateTransformer) traceInitializedSystem() error {
	tags := p.owner.contexts[0].config.ModelEvaluator.Trace
	if tags == nil {
		tags = p.manager.BorrowSourceTraceTags()
	}
	err := tags.EmitConditional("spacer", "init_rules", "src/muz/spacer/spacer_context.cpp", 1973,
		func() bool { return true }, func(out io.Writer) error {
			if len(p.rules) != 0 {
				_, _ = io.WriteString(out, "rules\n")
			}
			for _, normalized := range p.rules {
				if err := normalized.rule.WriteFormula(out); err != nil {
					if !tags.IsStreamFailure(err) {
						return err
					}
				}
				_, _ = io.WriteString(out, "\n")
			}
			_, _ = io.WriteString(out, "transition\n")
			transition, err := p.manager.RefGet(p.transition)
			if err != nil {
				return err
			}
			printer, err := ast.NewSimpleASTPrinterContext(p.manager, out, out)
			if err != nil {
				return err
			}
			if err = printer.DisplayExpr(out, transition, 0); err != nil {
				if !tags.IsStreamFailure(err) {
					return err
				}
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
