package smtcore

// Source: retained TRACE/CTRACE and SASSERT diagnostics in
// src/smt/theory_arith_eq.h.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *SMTLinearArithmeticTheory) arithmeticEqualityTraceTags() *diagnostic.TraceTags {
	if t != nil && t.equalityAdapter != nil && t.equalityAdapter.trace != nil {
		return t.equalityAdapter.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (t *SMTLinearArithmeticTheory) emitArithmeticEquality(tag, function string, line int, condition func() bool, body func(io.Writer) error) error {
	tags := t.arithmeticEqualityTraceTags()
	var operandErr error
	err := tags.EmitConditional(tag, function, "src/smt/theory_arith_eq.h", line, condition, func(out io.Writer) error {
		operandErr = body(assertedTraceWriter{out})
		return operandErr
	})
	if operandErr != nil {
		return operandErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func arithmeticEqualityAssertion(line int32, condition string, test bool) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled || test {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, "src/smt/theory_arith_eq.h", line, condition)
}

func (t *SMTLinearArithmeticTheory) displayArithmeticEqualityVariable(out io.Writer, column uint32) error {
	variable, found := t.theoryVariable(column)
	if !found {
		return ast.ErrASTDomain
	}
	node, err := t.Enode(variable)
	if err != nil {
		return err
	}
	if err = smtWriteTheoryText(out, fmt.Sprintf("v%d ", variable)); err != nil {
		return err
	}
	return smtArithEqDisplayExpression(t.state.manager, out, node.owner)
}

func (t *SMTLinearArithmeticTheory) displayArithmeticEqualityPair(out io.Writer, first, second uint32) error {
	if err := t.displayArithmeticEqualityVariable(out, first); err != nil {
		return err
	}
	if err := smtWriteTheoryText(out, " = "); err != nil {
		return err
	}
	return t.displayArithmeticEqualityVariable(out, second)
}

func displayArithmeticEqualityForm(out io.Writer, form *smtLinearForm) error {
	if form == nil {
		return ast.ErrASTHandle
	}
	if err := smtWriteTheoryText(out, fmt.Sprintf("infinity=%s constant=%s epsilon=%s", form.infinity.RatString(), form.constant.RatString(), form.infinitesimal.RatString())); err != nil {
		return err
	}
	variables := sortedSMTLinearFormVariables(form)
	for _, variable := range variables {
		if err := smtWriteTheoryText(out, fmt.Sprintf(" %+s*v%d", form.terms[variable].RatString(), variable)); err != nil {
			return err
		}
	}
	return smtWriteTheoryText(out, "\n")
}

func sortedSMTLinearFormVariables(form *smtLinearForm) []uint32 {
	variables := make([]uint32, 0, len(form.terms))
	for variable := range form.terms {
		variables = append(variables, variable)
	}
	for i := 1; i < len(variables); i++ {
		for j := i; j > 0 && variables[j] < variables[j-1]; j-- {
			variables[j], variables[j-1] = variables[j-1], variables[j]
		}
	}
	return variables
}
