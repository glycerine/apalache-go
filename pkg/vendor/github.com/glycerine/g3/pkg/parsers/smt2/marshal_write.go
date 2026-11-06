package smt2

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// Marshal writes the self-contained SMT-LIB benchmark form of expression.
// For Boolean expressions this includes declarations, an assertion unless the
// expression is true, and check-sat, exactly as ast_smt_pp::display_smt2.
func Marshal(out io.Writer, expression ast.ASTHandle, manager *ast.ASTManager) error {
	if out == nil || manager == nil || expression.IsNull() {
		return ErrMarshalInput
	}
	printer, err := ast.NewLegacySMTPrinter(manager)
	if err != nil {
		return err
	}
	return printer.DisplayBenchmark(out, expression, ast.LegacySMTBenchmarkConfig{})
}
