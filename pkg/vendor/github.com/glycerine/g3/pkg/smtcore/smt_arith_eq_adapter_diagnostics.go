package smtcore

// Source: retained TRACE/CTRACE and SASSERT diagnostics in
// src/smt/arith_eq_adapter.cpp:84-236.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func smtArithEqTrace(tags *diagnostic.TraceTags, tag, function string, line int, condition func() bool, body func(io.Writer) error) error {
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	var operandErr error
	err := tags.EmitConditional(tag, function, "src/smt/arith_eq_adapter.cpp", line, condition, func(out io.Writer) error {
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

func smtArithEqAssertion(line int32, condition string, test bool) error {
	d := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(d)
	if err != nil || !enabled || test {
		return err
	}
	return diagnostic.DebugAssertionFailure(d, "src/smt/arith_eq_adapter.cpp", line, condition)
}

func smtArithEqDisplayExpression(manager *ast.ASTManager, out io.Writer, expression ast.ASTHandle) error {
	printer, err := ast.NewSimpleASTPrinterContext(manager, out, out)
	if err != nil {
		return err
	}
	return printer.DisplayExpr(out, expression, 0)
}

func smtArithEqDisplayLiteral(out io.Writer, literal SMTLiteral) error {
	switch literal {
	case SMTTrueLiteral:
		return smtWriteTheoryText(out, "true")
	case SMTFalseLiteral:
		return smtWriteTheoryText(out, "false")
	}
	if literal.Sign() {
		return smtWriteTheoryText(out, fmt.Sprintf("(not p%d)", literal.Var()))
	}
	return smtWriteTheoryText(out, fmt.Sprintf("p%d", literal.Var()))
}

func smtArithEqDisplayBounded(manager *ast.ASTManager, out io.Writer, expression ast.ASTHandle, depth uint32) (result error) {
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	if err = manager.DisplayBoundedLowLevel(stream, expression, depth); err != nil {
		return err
	}
	text, err := stream.String()
	if err != nil {
		return err
	}
	return smtWriteTheoryText(out, text)
}

func smtArithEqTracePair(tags *diagnostic.TraceTags, manager *ast.ASTManager, function string, line int, tag string, leftVar, rightVar SMTTheoryVar, left, right *SMTEnode, disequality bool) error {
	return smtArithEqTrace(tags, tag, function, line, func() bool { return true }, func(out io.Writer) error {
		leftID, err := left.OwnerID()
		if err != nil {
			return err
		}
		rightID, err := right.OwnerID()
		if err != nil {
			return err
		}
		op := " = "
		if disequality {
			op = " != "
		}
		return smtWriteTheoryText(out, fmt.Sprintf("v%d%sv%d #%d%s#%d\n", leftVar, op, rightVar, leftID, op, rightID))
	})
}

func smtArithEqTraceContext(tags *diagnostic.TraceTags, tag, function string, line int, state *SMTInternalizerState) error {
	return smtArithEqTrace(tags, tag, function, line, func() bool { return true }, func(out io.Writer) error {
		return state.DisplayContext(out)
	})
}
