package spacer

// Source: src/muz/spacer/spacer_context.cpp pred_transformer::find_rule TRACE.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *PredicateTransformer) traceRuleSelectionCandidates() error {
	tags := p.owner.contexts[0].config.ModelEvaluator.Trace
	if tags == nil {
		tags = p.manager.BorrowSourceTraceTags()
	}
	err := tags.EmitConditional("spacer_verbose", "find_rule", "src/muz/spacer/spacer_context.cpp", 207,
		func() bool { return true }, func(out io.Writer) error {
			printer, err := ast.NewSimpleASTPrinterContext(p.manager, out, out)
			if err != nil {
				return err
			}
			for _, normalized := range p.rules {
				if normalized.tag.IsNull() {
					continue
				}
				if err = printer.DisplayExpr(out, normalized.tag, 0); err != nil {
					if !tags.IsStreamFailure(err) {
						return err
					}
				}
				_, _ = io.WriteString(out, ":\n")
				if normalized.rule != nil {
					if err = normalized.rule.WriteFormula(out); err != nil {
						if !tags.IsStreamFailure(err) {
							return err
						}
					}
					_, _ = io.WriteString(out, "\n")
				}
			}
			return nil
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
