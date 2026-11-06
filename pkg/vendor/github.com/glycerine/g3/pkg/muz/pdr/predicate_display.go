package pdr

// Source: src/muz/pdr/pdr_context.cpp pred_transformer::display, consumed by
// context initialization traces and context::display.
import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// WriteDisplay writes the source rule and transition diagnostic. The writer
// and transformer are borrowed; presentation does not change search state.
func (p *PredicateTransformer) WriteDisplay(out io.Writer) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	transition, err := p.Transition()
	if err != nil {
		return err
	}
	owner := &pdrModelExportOwner{manager: p.manager}
	defer func() { resultErr = owner.dispose(resultErr) }()
	printer, err := ast.NewSimpleASTPrinterContext(p.manager, out, out)
	if err != nil {
		return err
	}
	if len(p.rules) != 0 {
		if _, err = io.WriteString(out, "rules\n"); err != nil {
			return err
		}
	}
	for _, normalized := range p.rules {
		formula, err := normalized.rule.ToFormula()
		if err != nil {
			return err
		}
		owner.references = append(owner.references, formula)
		handle, err := p.manager.RefGet(formula)
		if err != nil {
			return err
		}
		if err = printer.DisplayExpr(out, handle, 0); err != nil {
			return err
		}
		if _, err = io.WriteString(out, "\n"); err != nil {
			return err
		}
		if err = owner.releaseLastReference(); err != nil {
			return err
		}
	}
	if _, err = io.WriteString(out, "transition\n"); err != nil {
		return err
	}
	if err = printer.DisplayExpr(out, transition, 0); err != nil {
		return err
	}
	_, err = io.WriteString(out, "\n")
	return err
}
