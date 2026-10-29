package ast

// Source: src/ast/expr_abstract.cpp:112-133; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// The source prints expr_ref values through its SMT2 printer. Keep each
// formatter call inside the trace body so a disabled tag does no AST work.
func (a *ExprAbstractor) traceExprAbstract(root ASTHandle, bound []ASTHandle, result ASTHandle, quantifier bool) error {
	m := a.manager
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("expr_abstract") {
		return nil
	}
	line, function := 113, "expr_abstract"
	if quantifier {
		line, function = 130, "mk_quantifier"
	}
	return tags.EmitConditional("expr_abstract", function, "src/ast/expr_abstract.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			c, p, g, err := m.wellSortedTraceParams(nil, nil, nil)
			if err != nil {
				return err
			}
			writeExpr := func(h ASTHandle, suffix string) error {
				body, err := m.wellSortedTraceSMT2(h, c, p, g)
				if err != nil {
					return err
				}
				_, err = io.WriteString(w, body+suffix)
				return err
			}
			if err = writeExpr(root, "\n"); err != nil {
				return err
			}
			if quantifier {
				for _, h := range bound {
					if err = writeExpr(h, " "); err != nil {
						return err
					}
				}
				if _, err = io.WriteString(w, "\n"); err != nil {
					return err
				}
			}
			return writeExpr(result, "\n")
		})
}
