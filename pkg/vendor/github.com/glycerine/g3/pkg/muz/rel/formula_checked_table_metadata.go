package rel

// Source: src/muz/rel/check_relation.cpp check_relation::display and
// check_relation::consistent_formula; check_relation.h precision/size forwarding.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// IsPrecise forwards the concrete table relation's exact finite semantics.
func (r *FormulaCheckedTable) IsPrecise() (bool, error) {
	if err := r.open(); err != nil {
		return false, err
	}
	return true, nil
}

// SizeEstimateRows forwards the implementation-under-check's exact row count.
func (r *FormulaCheckedTable) SizeEstimateRows() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.table.Table().SizeEstimateRows()
}

// Display prints the wrapped relation followed by the checker's recorded
// formula, matching check_relation::display ordering.
func (r *FormulaCheckedTable) Display(out io.Writer) error {
	if err := r.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if err := r.table.Display(out); err != nil {
		return err
	}
	formula, err := r.manager.RefGet(r.formula)
	if err != nil {
		return err
	}
	printer, err := ast.NewSimpleASTPrinterContext(r.manager, out, out)
	if err != nil {
		return err
	}
	if err = printer.DisplayExpr(out, formula, 0); err != nil {
		return err
	}
	_, err = io.WriteString(out, "\n")
	return err
}

// ConsistentFormula follows the source diagnostic boundary: identity is fast,
// while a differing concrete formula emits the relation diagnostic but does
// not turn the mismatch into an operation failure.
func (r *FormulaCheckedTable) ConsistentFormula(out io.Writer) (consistent bool, resultErr error) {
	if err := r.open(); err != nil {
		return false, err
	}
	actual, err := r.table.ToFormula()
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, r.manager.DelRef(&actual)) }()
	actualHandle, err := r.manager.RefGet(actual)
	if err != nil {
		return false, err
	}
	expected, err := r.manager.RefGet(r.formula)
	if err != nil {
		return false, err
	}
	if actualHandle == expected {
		return true, nil
	}
	if out == nil {
		return false, ast.ErrASTHandle
	}
	if _, err = io.WriteString(out, "relation does not have a consistent formula\n"); err != nil {
		return false, err
	}
	return false, r.Display(out)
}
