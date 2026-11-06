package pdr

// Source: pred_transformer::initialize, pdr_context.cpp:519-521.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// A source ostream with badbit still evaluates later insertion operands.
type pdrTraceOstream struct{ out io.Writer }

func (w pdrTraceOstream) Write(body []byte) (int, error) {
	_, _ = w.out.Write(body)
	return len(body), nil
}

func (p *PredicateTransformer) traceInitialSystem(initial, transition ast.ASTHandle) error {
	tags := p.manager.BorrowSourceTraceTags()
	err := tags.EmitConditional("pdr", "initialize", "src/muz/pdr/pdr_context.cpp", 519,
		func() bool { return true }, func(raw io.Writer) error {
			out := pdrTraceOstream{out: raw}
			printer, x := ast.NewSimpleASTPrinterContext(p.manager, nil, nil)
			if x != nil {
				return x
			}
			if _, x = io.WriteString(out, "Initial state: "); x != nil {
				return x
			}
			if x = printer.DisplayExpr(out, initial, 0); x != nil {
				return x
			}
			if _, x = io.WriteString(out, "\nTransition:    "); x != nil {
				return x
			}
			if x = printer.DisplayExpr(out, transition, 0); x != nil {
				return x
			}
			_, x = io.WriteString(out, "\n")
			return x
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
