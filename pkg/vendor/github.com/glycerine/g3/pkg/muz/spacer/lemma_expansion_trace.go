package spacer

// Source: src/muz/spacer/spacer_util.cpp expand_literals TRACE sites.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *PredicateTransformer) traceLiteralExpansion(conjs []ast.ASTHandle, begin bool) error {
	tags := p.manager.BorrowSourceTraceTags()
	line, label := 677, "begin expand\n"
	if !begin {
		line, label = 722, "end expand\n"
	}
	err := tags.EmitConditional("spacer_expand", "expand_literals", "src/muz/spacer/spacer_util.cpp", line,
		func() bool { return true }, func(out io.Writer) error {
			printer, err := ast.NewSimpleASTPrinterContext(p.manager, out, out)
			if err != nil {
				return err
			}
			_, _ = io.WriteString(out, label)
			for _, literal := range conjs {
				if err := printer.DisplayExpr(out, literal, 0); err != nil {
					if !tags.IsStreamFailure(err) {
						return err
					}
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
