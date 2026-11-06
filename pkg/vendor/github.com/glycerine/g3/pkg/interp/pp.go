package interp

// Source: src/interp/iz3pp.cpp; src/interp/iz3pp.h.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// BadTreeError identifies a malformed tree-interpolation specification.
type BadTreeError struct{ cause Error }

func newBadTreeError() BadTreeError                     { return BadTreeError{cause: NewError("iz3pp_bad_tree")} }
func (e BadTreeError) Error() string                    { return e.cause.Error() }
func (e BadTreeError) Message() string                  { return e.cause.Message() }
func (e BadTreeError) ErrorCode() z3exception.ErrorCode { return e.cause.ErrorCode() }
func (e BadTreeError) HasErrorCode() bool               { return e.cause.HasErrorCode() }

func interpWrite(out io.Writer, text string) error {
	if out == nil {
		return NewError("invalid interpolation output")
	}
	n, err := io.WriteString(out, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}

func printInterpolationTree(m *ast.ASTManager, tree ast.ASTHandle, names map[ast.ASTHandle]string, out io.Writer) error {
	if name, found := names[tree]; found {
		return interpWrite(out, name)
	}
	kind, err := m.GetKind(tree)
	if err != nil || kind != ast.ASTApp {
		return newBadTreeError()
	}
	family, err := m.GetFamilyID(tree)
	if err != nil {
		return err
	}
	op, err := m.GetDeclKind(tree)
	if err != nil {
		return err
	}
	if family != m.BasicFamilyID() {
		return newBadTreeError()
	}
	switch op {
	case ast.ASTOpAnd:
		if err = interpWrite(out, "(and"); err != nil {
			return err
		}
		count, e := m.GetNumArgs(tree)
		if e != nil {
			return e
		}
		for i := uint32(0); i < count; i++ {
			if err = interpWrite(out, " "); err != nil {
				return err
			}
			arg, e := m.GetArg(tree, i)
			if e != nil {
				return e
			}
			if err = printInterpolationTree(m, arg, names, out); err != nil {
				return err
			}
		}
		return interpWrite(out, ")")
	case ast.ASTOpInterp:
		count, e := m.GetNumArgs(tree)
		if e != nil {
			return e
		}
		if count != 1 {
			return newBadTreeError()
		}
		arg, e := m.GetArg(tree, 0)
		if e != nil {
			return e
		}
		if err = interpWrite(out, "(interp "); err != nil {
			return err
		}
		if err = printInterpolationTree(m, arg, names, out); err != nil {
			return err
		}
		return interpWrite(out, ")")
	default:
		return newBadTreeError()
	}
}

// PrettyPrintProblem emits the retained interpolation-specific SMT problem.
// It borrows the manager and every AST handle for the duration of the call.
func PrettyPrintProblem(m *ast.ASTManager, constraints []ast.ASTHandle, tree ast.ASTHandle, out io.Writer) error {
	if m == nil || out == nil {
		return NewError("invalid interpolation pretty-printer")
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return err
	}
	if err := interpWrite(out, "(set-option :produce-interpolants true)\n"); err != nil {
		return err
	}
	declarations, err := ast.NewASTPPUtil(m)
	if err != nil {
		return err
	}
	if err = declarations.CollectAll(constraints); err != nil {
		return err
	}
	if err = declarations.DisplayDeclarations(out); err != nil {
		return err
	}
	printer, err := ast.NewLegacySMTPrinter(m)
	if err != nil {
		return err
	}
	names := make(map[ast.ASTHandle]string, len(constraints))
	for i, constraint := range constraints {
		name := fmt.Sprintf("f!%d", i+1)
		names[constraint] = name
		if err = interpWrite(out, "(assert (! "); err != nil {
			return err
		}
		if err = printer.DisplayExpr(out, constraint, nil); err != nil {
			return err
		}
		if err = interpWrite(out, " :named "+name+"))\n"); err != nil {
			return err
		}
	}
	if err = interpWrite(out, "(check-sat)\n(get-interpolant "); err != nil {
		return err
	}
	if err = printInterpolationTree(m, tree, names, out); err != nil {
		return err
	}
	return interpWrite(out, ")\n")
}
