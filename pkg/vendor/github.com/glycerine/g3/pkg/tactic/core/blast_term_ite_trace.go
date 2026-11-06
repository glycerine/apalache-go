package core

// Source: blast_term_ite_tactic.cpp:66.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

type blastTermITETraceOstream struct{ out io.Writer }

func (w blastTermITETraceOstream) Write(p []byte) (int, error) {
	_, _ = w.out.Write(p)
	return len(p), nil
}

func (c *blastTermITEConfiguration) traceApplication(decl ast.ASTHandle, args []ast.ASTHandle, result ast.ASTRef) error {
	m := c.owner.manager
	tags := m.BorrowSourceTraceTags()
	err := tags.EmitConditional("blast_term_ite", "mk_app_core", "src/tactic/core/blast_term_ite_tactic.cpp", 66,
		func() bool { return true }, func(raw io.Writer) error {
			app, x := m.MkApp(decl, args)
			if x != nil {
				return x
			}
			if x = m.SetRef(result, app); x != nil {
				return x
			}
			printer, x := ast.NewSimpleASTPrinterContext(m, nil, nil)
			if x != nil {
				return x
			}
			out := blastTermITETraceOstream{out: raw}
			if x = printer.DisplayExpr(out, app, 0); x != nil {
				return x
			}
			_, _ = io.WriteString(out, "\n")
			return nil
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
