package ackermannization

// Source: src/ackermannization retained diagnostics.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func emitAck(tags *diagnostic.TraceTags, tag, function, file string, line int, body func(io.Writer) error) error {
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional(tag, function, file, line, func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func writeAckExpr(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, out io.Writer, h ast.ASTHandle) (err error) {
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, stream.Close()) }()
	kind, err := m.GetKind(h)
	if err != nil {
		return err
	}
	var f ast.ASTHandle
	if kind == ast.ASTFuncDecl {
		f, err = m.SMT2PPDeclareFun(h)
	} else {
		f, err = m.SMT2PPExprTree(h, nil)
	}
	if err != nil {
		return err
	}
	fm, err := m.GetFormatManager()
	if err != nil {
		return err
	}
	held, err := fm.NewRef(f)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, fm.DelRef(&held)) }()
	p := c.EmptyValue()
	if err = m.PrintFormat(stream, f, c, &p, g); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, text)
	return err
}
func (c *ModelConverter) BindTrace(tags *diagnostic.TraceTags) error {
	if err := c.open(); err != nil {
		return err
	}
	c.trace = tags
	return nil
}
func (c *ModelConverter) emit(function string, line int, body func(io.Writer) error) error {
	return emitAck(c.trace, "ackr_model", function, "src/ackermannization/ackr_model_converter.cpp", line, body)
}
func (c *ModelConverter) traceEntry(term, value ast.ASTHandle) error {
	return c.emit("add_entry", 113, func(w io.Writer) error {
		if _, err := io.WriteString(w, "add_entry"); err != nil {
			return err
		}
		if err := writeAckExpr(c.manager, c.services.Context, c.services.Globals, w, term); err != nil {
			return err
		}
		if _, err := io.WriteString(w, "->"); err != nil {
			return err
		}
		if err := writeAckExpr(c.manager, c.services.Context, c.services.Globals, w, value); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
func (c *ModelConverter) traceNewEntry(decl ast.ASTHandle, args []ast.ASTHandle, value ast.ASTHandle) error {
	return c.emit("add_entry", 137, func(w io.Writer) error {
		if err := writeAckExpr(c.manager, c.services.Context, c.services.Globals, w, decl); err != nil {
			return err
		}
		if _, err := io.WriteString(w, " args: \n"); err != nil {
			return err
		}
		for _, arg := range args {
			if err := writeAckExpr(c.manager, c.services.Context, c.services.Globals, w, arg); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, " -> "); err != nil {
			return err
		}
		if err := writeAckExpr(c.manager, c.services.Context, c.services.Globals, w, value); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\n")
		return err
	})
}
