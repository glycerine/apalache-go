package arith

// Source: src/tactic/arith/fm_tactic.cpp:246-262.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func fmDisplayWrite(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err == nil && n != len(text) {
		err = io.ErrShortWrite
	}
	return err
}
func (c *FMModelConverter) Display(w io.Writer) error {
	if err := c.open(); err != nil {
		return err
	}
	if w == nil {
		return arena.ErrInvalidSlot
	}
	if err := fmDisplayWrite(w, "(fm-model-converter"); err != nil {
		return err
	}
	for i, x := range c.variables {
		symbol, err := c.manager.GetASTName(x)
		if err != nil {
			return err
		}
		name, err := c.manager.Symbols.Display(symbol)
		if err != nil {
			return err
		}
		if err = fmDisplayWrite(w, "\n("+name); err != nil {
			return err
		}
		for _, clause := range c.clauses[i] {
			if err = fmDisplayWrite(w, "\n  "); err != nil {
				return err
			}
			body, err := c.displayClause(clause)
			if err != nil {
				return err
			}
			if err = fmDisplayWrite(w, body); err != nil {
				return err
			}
		}
		if err = fmDisplayWrite(w, ")"); err != nil {
			return err
		}
	}
	return fmDisplayWrite(w, ")\n")
}
func (c *FMModelConverter) displayClause(clause ast.ASTHandle) (body string, err error) {
	return fmExpressionText(c.manager, c.services, clause, 2)
}
func fmExpressionText(m *ast.ASTManager, services model.ModelEvaluatorConfig, clause ast.ASTHandle, indent uint32) (body string, err error) {
	empty := services.Context.EmptyValue()
	doc, err := m.SMT2PPExprTreeWithParams(clause, nil, services.Context, services.Globals, &empty)
	if err != nil {
		return "", err
	}
	ref, err := m.Format.NewRef(doc)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, m.Format.DelRef(&ref)) }()
	doc, err = m.FormatIndent(indent, doc)
	if err != nil {
		return "", err
	}
	if err = m.Format.SetRef(ref, doc); err != nil {
		return "", err
	}
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, out.Close()) }()
	if err = m.PrintFormat(out, doc, services.Context, &empty, services.Globals); err != nil {
		return "", err
	}
	return out.String()
}
