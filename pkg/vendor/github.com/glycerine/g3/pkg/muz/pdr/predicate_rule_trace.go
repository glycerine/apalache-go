package pdr

// Source: pred_transformer::init_rule, pdr_context.cpp:602,616.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (b *predicateRuleBuilder) traceRuleExpressions(line int, expressions ...ast.ASTHandle) error {
	tags := b.transformer.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("pdr", "init_rule", "src/muz/pdr/pdr_context.cpp", line,
		func() bool { return true }, func(raw io.Writer) error {
			out := pdrTraceOstream{out: raw}
			printer, x := ast.NewSimpleASTPrinterContext(b.transformer.manager, nil, nil)
			if x != nil {
				return x
			}
			for _, expression := range expressions {
				if x = printer.DisplayExpr(out, expression, 0); x != nil {
					return x
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
