package rel

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// Display writes the external relation expression followed by the source
// newline, matching external_relation::display.
func (r *ExternalRelation) Display(out io.Writer) error {
	if err := r.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	expression, err := r.Relation()
	if err != nil {
		return err
	}
	printer, err := ast.NewSimpleASTPrinterContext(r.ast, out, nil)
	if err != nil {
		return err
	}
	if err = printer.DisplayExpr(out, expression, 0); err != nil {
		return err
	}
	_, err = io.WriteString(out, "\n")
	return err
}

// DisplayTuples delegates to Display just as the source relation does; the
// predicate is intentionally unused.
func (r *ExternalRelation) DisplayTuples(_ ast.ASTHandle, out io.Writer) error {
	return r.Display(out)
}
