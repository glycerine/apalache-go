package smtcore

// Source: src/smt/smt_context_pp.cpp:89-91,135-148,264-284;
// src/smt/asserted_formulas.cpp display_ll.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// DisplayDefinitions shares one low-level visited set across Boolean variables,
// enodes and assertion roots, in source order. The reserved immediate true
// variable in Go has no boolExpr entry and needs no expression definition.
func (s *SMTInternalizerState) DisplayDefinitions(out io.Writer) (result error) {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	var roots []ast.ASTHandle
	for _, expression := range s.boolExpr {
		if !expression.IsNull() {
			roots = append(roots, expression)
		}
	}
	for _, node := range s.enodeStack {
		roots = append(roots, node.owner)
	}
	var assertions []SMTKernelAssertion
	if s.assertedFormulas != nil {
		assertions = *s.assertedFormulas
		for _, row := range assertions {
			roots = append(roots, row.Formula)
		}
	}
	if len(roots) == 0 {
		return nil
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: s.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	if err = s.manager.DisplayLowLevelExpressionDefinitions(stream, roots); err != nil {
		return err
	}
	if len(assertions) != 0 {
		if err = stream.AppendCString("asserted formulas:\n"); err != nil {
			return err
		}
		for _, row := range assertions {
			id, err := s.manager.GetASTID(row.Formula)
			if err != nil {
				return err
			}
			if err = stream.AppendCString(fmt.Sprintf("#%d ", id)); err != nil {
				return err
			}
		}
		if err = stream.AppendCString("\n"); err != nil {
			return err
		}
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	return smtWriteTheoryText(out, text)
}

func (s *SMTInternalizerState) DisplayHotBooleanVariables(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if err := smtWriteTheoryText(out, "hot bool vars:\n"); err != nil {
		return err
	}
	for variable := SMTFirstBoolVar; int(variable) < len(s.boolExpr); variable++ {
		value := s.activity[variable] / s.activityIncrement
		if !(value > 10.0) {
			continue
		}
		id, err := s.manager.GetASTID(s.boolExpr[variable])
		if err != nil {
			return err
		}
		// ostream defaultfloat uses six significant digits. Width applies to
		// each numeric operand; AST IDs are left-aligned and values right.
		if err = smtWriteTheoryText(out, fmt.Sprintf("#%-5d  %12.6g  %12.6g\n", id, s.activity[variable], value)); err != nil {
			return err
		}
	}
	return nil
}
