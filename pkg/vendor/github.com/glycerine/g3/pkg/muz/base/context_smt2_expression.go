package base

// Source: dl_context.cpp display_smt2's PP macro selects mk_smt_pp versus
// ast_smt2_pp; format and stream owners are scoped to each insertion.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (s *RuleSet) writeContextExpression(out io.Writer, printer *ast.LegacySMTPrinter, h ast.ASTHandle, low bool) (resultErr error) {
	if low {
		return printer.DisplayExpr(out, h, nil)
	}
	m := s.manager.manager
	owner := &contextSMT2Owner{manager: m}
	defer func() { resultErr = owner.finish(resultErr) }()
	var err error
	owner.formats, err = m.GetFormatManager()
	if err != nil {
		return err
	}
	owner.format, err = owner.formats.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	c, g, err := m.RewriterParameterServices()
	if err != nil {
		return err
	}
	var format ast.ASTHandle
	if c != nil && g != nil {
		format, err = m.SMT2PPExprTreeWithParams(h, nil, c, g, nil)
	} else {
		format, err = m.SMT2PPExprTree(h, nil)
	}
	if !format.IsNull() {
		err = errors.Join(err, owner.formats.SetRef(owner.format, format))
	}
	if err != nil {
		return err
	}
	return s.writeContextFormat(out, owner, format)
}

func (s *RuleSet) writeContextSort(out io.Writer, sort ast.ASTHandle) (resultErr error) {
	m := s.manager.manager
	owner := &contextSMT2Owner{manager: m}
	defer func() { resultErr = owner.finish(resultErr) }()
	var err error
	owner.formats, err = m.GetFormatManager()
	if err != nil {
		return err
	}
	owner.format, err = owner.formats.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	format, err := m.SMT2PPSort(sort)
	if !format.IsNull() {
		err = errors.Join(err, owner.formats.SetRef(owner.format, format))
	}
	if err != nil {
		return err
	}
	return s.writeContextFormat(out, owner, format)
}

func (s *RuleSet) writeContextFormat(out io.Writer, owner *contextSMT2Owner, format ast.ASTHandle) error {
	m := s.manager.manager
	var err error
	owner.stream, err = utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.Budget()})
	if err != nil {
		return err
	}
	c, g, err := m.RewriterParameterServices()
	if err != nil {
		return err
	}
	if c != nil && g != nil {
		empty := c.EmptyValue()
		err = m.PrintFormat(owner.stream, format, c, &empty, g)
	} else {
		err = m.PrintFormat(owner.stream, format, nil, nil, nil)
	}
	if err != nil {
		return err
	}
	body, err := owner.stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, body)
	return err
}
