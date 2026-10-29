package interp

// Source checkpoint: src/interp/iz3mgr.cpp display helpers.

import (
	"io"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (m *Manager) PrintExpr(out io.Writer, expression ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	if out == nil {
		return NewError("invalid interpolation output")
	}
	printer, err := ast.NewLegacySMTPrinter(m.ast)
	if err != nil {
		return err
	}
	return printer.DisplayExpr(out, expression, nil)
}

func (m *Manager) PrintClause(out io.Writer, clause []ast.ASTHandle) error {
	if err := interpWrite(out, "("); err != nil {
		return err
	}
	for i, expression := range clause {
		if i != 0 {
			if err := interpWrite(out, ","); err != nil {
				return err
			}
		}
		if err := m.PrintExpr(out, expression); err != nil {
			return err
		}
	}
	return interpWrite(out, ")")
}

func (m *Manager) PrintLiteral(out io.Writer, literal ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	isNot, absolute, err := m.ast.IsNot(literal)
	if err != nil {
		return err
	}
	if !isNot {
		absolute = literal
	}
	op, err := m.Operator(absolute)
	if err != nil {
		return err
	}
	if op != OperatorAnd && op != OperatorOr && op != OperatorIff {
		return m.PrintExpr(out, literal)
	}
	if isNot {
		if err = interpWrite(out, "~"); err != nil {
			return err
		}
	}
	if err = interpWrite(out, "["); err != nil {
		return err
	}
	if err = m.PrintExpr(out, absolute); err != nil {
		return err
	}
	return interpWrite(out, "]")
}

func (m *Manager) Show(out io.Writer, expression ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	if expression.IsNull() {
		return interpWrite(out, "(null)\n")
	}
	if err := m.PrintExpr(out, expression); err != nil {
		return err
	}
	return interpWrite(out, "\n")
}

func (m *Manager) ShowSymbol(out io.Writer, declaration ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	if out == nil {
		return NewError("invalid interpolation output")
	}
	printer, err := ast.NewLegacySMTPrinter(m.ast)
	if err != nil {
		return err
	}
	if err = printer.DisplayAST(out, declaration, 0, nil); err != nil {
		return err
	}
	return interpWrite(out, "\n")
}

func (m *Manager) ShowClause(out io.Writer, clause []ast.ASTHandle) error {
	if err := m.PrintClause(out, clause); err != nil {
		return err
	}
	return interpWrite(out, "\n")
}

func (m *Manager) PrintSATProblem(out io.Writer, expression ast.ASTHandle) error {
	if err := m.open(); err != nil {
		return err
	}
	if out == nil {
		return NewError("invalid interpolation output")
	}
	printer, err := ast.NewLegacySMTPrinter(m.ast)
	if err != nil {
		return err
	}
	return printer.DisplayBenchmark(out, expression, ast.LegacySMTBenchmarkConfig{})
}

func prettyDelimiter(s string, position int) int {
	level := 0
	for position < len(s) {
		switch s[position] {
		case '(':
			level++
		case ')':
			level--
		}
		if level < 0 || level == 0 && s[position] == ',' {
			break
		}
		position++
	}
	return position
}

// PrettyPrint preserves iz3mgr's 79-column comma/parenthesis formatter.
func PrettyPrint(out io.Writer, source string) error {
	if out == nil {
		return NewError("invalid interpolation output")
	}
	const (
		columns    = 79
		indentStep = 2
	)
	currentIndent, indent, column, position := 0, 0, 0, 0
	newline := func() error {
		return interpWrite(out, "\n"+strings.Repeat(" ", indent))
	}
	for position < len(source) {
		delimiter := prettyDelimiter(source, position)
		if source[position] != ')' && source[position] != ',' && currentIndent > indent {
			if err := newline(); err != nil {
				return err
			}
			currentIndent, column = indent, indent
			continue
		}
		if column+delimiter-position > columns {
			if column > indent {
				if err := newline(); err != nil {
					return err
				}
				currentIndent, column = indent, indent
				continue
			}
			if offset := strings.IndexByte(source[position:], '('); offset >= 0 {
				end := position + offset + 1
				if err := interpWrite(out, source[position:end]); err != nil {
					return err
				}
				indent += indentStep
				if column != 0 {
					if err := newline(); err != nil {
						return err
					}
				}
				currentIndent, position, column = indent, end, indent
				continue
			}
		}
		end := delimiter + 1
		if end > len(source) {
			end = len(source)
		}
		if err := interpWrite(out, source[position:end]); err != nil {
			return err
		}
		column += end - position
		position = end
		if delimiter < len(source) && source[delimiter] == ')' {
			indent -= indentStep
		}
	}
	return nil
}
