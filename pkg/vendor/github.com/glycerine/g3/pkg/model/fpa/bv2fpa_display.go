package fpa

// Source: src/ast/fpa/bv2fpa_converter.cpp:474-503.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

// Display writes the source converter cache form used by the FPA model
// converter's diagnostic display.
func (c *Converter) Display(out io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	printer, err := ast.NewSimpleASTPrinterContext(c.manager, out, out)
	if err != nil {
		return err
	}
	write := func(text string) error {
		_, err := io.WriteString(out, text)
		return err
	}
	name := func(declaration ast.ASTHandle) (string, error) {
		symbol, err := c.manager.GetASTName(declaration)
		if err != nil {
			return "", err
		}
		return c.manager.Symbols.Display(symbol)
	}
	expressionEntry := func(declaration, value ast.ASTHandle) error {
		text, err := name(declaration)
		if err != nil {
			return err
		}
		if err = write("\n  (" + text + " "); err != nil {
			return err
		}
		if err = printer.DisplayExpr(out, value, 0); err != nil {
			return err
		}
		return write(")")
	}
	declarationEntry := func(declaration, value ast.ASTHandle) error {
		text, err := name(declaration)
		if err != nil {
			return err
		}
		if err = write("\n  (" + text + " "); err != nil {
			return err
		}
		if err = printer.DisplayFuncDecl(out, value, 0); err != nil {
			return err
		}
		return write(")")
	}
	for declaration, value := range c.constants {
		if err = expressionEntry(declaration, value); err != nil {
			return err
		}
	}
	for declaration, value := range c.roundingConstants {
		if err = expressionEntry(declaration, value); err != nil {
			return err
		}
	}
	for declaration, value := range c.convertedUFs {
		if err = declarationEntry(declaration, value); err != nil {
			return err
		}
	}
	for declaration, values := range c.minMaxSigns {
		text, e := name(declaration)
		if e != nil {
			return e
		}
		if e = write("\n  (" + text + " "); e != nil {
			return e
		}
		if e = printer.DisplayExpr(out, values[0], 0); e != nil {
			return e
		}
		if e = write("; "); e != nil {
			return e
		}
		if e = printer.DisplayExpr(out, values[1], 0); e != nil {
			return e
		}
		if e = write(")"); e != nil {
			return e
		}
	}
	return nil
}
