package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:63,668,702,706,736,748,753;
// src/util/trace.h:40-46.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (r *formulaProcessor) formulaTraceTags() *diagnostic.TraceTags {
	if r.trace != nil {
		return r.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (r *formulaProcessor) traceFormulaText(h ast.ASTHandle) (body string, result error) {
	return formulaTraceText(r.manager, h)
}

func formulaTraceText(manager *ast.ASTManager, h ast.ASTHandle) (body string, result error) {
	format, e := manager.SMT2PPExprTree(h, nil)
	if e != nil {
		return "", e
	}
	out, e := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: manager.Budget()})
	if e != nil {
		return "", e
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	if e := manager.PrintFormat(out, format, nil, nil, nil); e != nil {
		return "", e
	}
	return out.String()
}

func (r *formulaProcessor) traceFormulas(line int, heading string, formulas []ast.ASTHandle) error {
	tags := r.formulaTraceTags()
	if !tags.ShouldTrace("demodulator") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("demodulator", "operator()", "src/tactic/ufbv/ufbv_rewriter.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, heading)
			for _, h := range formulas {
				body, e := r.traceFormulaText(h)
				if e != nil {
					operandErr = e
					return e
				}
				_, _ = io.WriteString(w, body+"\n")
			}
			return nil
		})
	return operandErr
}

func (r *formulaProcessor) traceDemodulatorApp(h ast.ASTHandle) (string, error) {
	decl, e := r.manager.GetDecl(h)
	if e != nil {
		return "", e
	}
	name, e := r.manager.GetASTName(decl)
	if e != nil {
		return "", e
	}
	text, e := r.manager.Symbols.Display(name)
	if e != nil {
		return "", e
	}
	depth, e := r.manager.GetDepth(h)
	if e != nil {
		return "", e
	}
	return text + "[" + strconv.FormatUint(uint64(depth), 10) + "]", nil
}

func (r *formulaProcessor) traceFoundDemodulator(large, small ast.ASTHandle) error {
	tags := r.formulaTraceTags()
	if tags.ShouldTrace("demodulator") {
		var operandErr error
		_ = tags.EmitConditional("demodulator", "operator()", "src/tactic/ufbv/ufbv_rewriter.cpp", 702,
			func() bool { return true }, func(w io.Writer) error {
				_, _ = io.WriteString(w, "Found demodulator: \n")
				lhs, e := r.traceFormulaText(large)
				if e != nil {
					operandErr = e
					return e
				}
				_, _ = io.WriteString(w, lhs+"\n ---> \n")
				rhs, e := r.traceFormulaText(small)
				if e != nil {
					operandErr = e
					return e
				}
				_, _ = io.WriteString(w, rhs+"\n")
				return nil
			})
		if operandErr != nil {
			return operandErr
		}
	}
	if !tags.ShouldTrace("demodulator_s") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("demodulator_s", "operator()", "src/tactic/ufbv/ufbv_rewriter.cpp", 706,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "Found demodulator: \n")
			lhs, e := r.traceDemodulatorApp(large)
			if e != nil {
				operandErr = e
				return e
			}
			_, _ = io.WriteString(w, lhs+" ---> ")
			kind, e := r.manager.GetKind(small)
			if e != nil {
				operandErr = e
				return e
			}
			var rhs string
			if kind == ast.ASTApp {
				rhs, e = r.traceDemodulatorApp(small)
			} else {
				rhs, e = r.traceFormulaText(small)
			}
			if e != nil {
				operandErr = e
				return e
			}
			_, _ = io.WriteString(w, rhs+"\n")
			return nil
		})
	return operandErr
}
