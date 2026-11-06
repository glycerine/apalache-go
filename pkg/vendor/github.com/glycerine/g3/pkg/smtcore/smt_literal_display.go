package smtcore

// Source: src/smt/smt_literal.cpp AST-aware display helpers and
// src/smt/smt_literal.h neg_literals.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func smtLiteralExpression(l SMTLiteral, expressions []ast.ASTHandle) (ast.ASTHandle, error) {
	variable := l.Var()
	if variable < 0 || int(variable) >= len(expressions) || expressions[variable].IsNull() {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return expressions[variable], nil
}

func (l SMTLiteral) Display(out io.Writer, manager *ast.ASTManager, expressions []ast.ASTHandle) error {
	if out == nil || manager == nil {
		return ast.ErrASTHandle
	}
	switch l {
	case SMTTrueLiteral:
		return smtWriteTheoryText(out, "true")
	case SMTFalseLiteral:
		return smtWriteTheoryText(out, "false")
	case SMTNullLiteral:
		return smtWriteTheoryText(out, "null")
	}
	expression, err := smtLiteralExpression(l, expressions)
	if err != nil {
		return err
	}
	out = smtContextDisplayWriter{out}
	printer, err := ast.NewSimpleASTPrinterContext(manager, out, out)
	if err != nil {
		return err
	}
	if l.Sign() {
		if err = smtWriteTheoryText(out, "(not "); err != nil {
			return err
		}
	}
	if err = printer.DisplayExpr(out, expression, 0); err != nil {
		return err
	}
	if l.Sign() {
		err = smtWriteTheoryText(out, ")")
	}
	return err
}

func (l SMTLiteral) DisplayCompact(out io.Writer, manager *ast.ASTManager, expressions []ast.ASTHandle) error {
	if out == nil || manager == nil {
		return ast.ErrASTHandle
	}
	if l == SMTTrueLiteral {
		_, err := io.WriteString(out, "true")
		return err
	}
	if l == SMTFalseLiteral {
		_, err := io.WriteString(out, "false")
		return err
	}
	expression, err := smtLiteralExpression(l, expressions)
	if err != nil {
		return err
	}
	id, err := manager.GetASTID(expression)
	if err != nil {
		return err
	}
	if l.Sign() {
		_, err = io.WriteString(out, "(not #"+strconv.FormatUint(uint64(id), 10)+")")
	} else {
		_, err = io.WriteString(out, "#"+strconv.FormatUint(uint64(id), 10))
	}
	return err
}

func DisplaySMTLiteralsCompact(out io.Writer, manager *ast.ASTManager, literals []SMTLiteral, expressions []ast.ASTHandle) error {
	for i, literal := range literals {
		if i != 0 {
			if _, err := io.WriteString(out, " "); err != nil {
				return err
			}
		}
		if err := literal.DisplayCompact(out, manager, expressions); err != nil {
			return err
		}
	}
	return nil
}

func DisplaySMTLiteralsVerbose(out io.Writer, manager *ast.ASTManager, literals []SMTLiteral, expressions []ast.ASTHandle, separator string) error {
	for i, literal := range literals {
		if i != 0 {
			if err := smtWriteTheoryText(out, separator); err != nil {
				return err
			}
		}
		if err := literal.Display(out, manager, expressions); err != nil {
			return err
		}
	}
	return nil
}

func NegateSMTLiterals(literals []SMTLiteral) []SMTLiteral {
	result := make([]SMTLiteral, len(literals))
	for i, literal := range literals {
		result[i] = literal.Negated()
	}
	return result
}
