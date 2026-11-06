package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:336,359,423,462; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (r *rewriteTraversal) traceCycle(app, child ast.ASTHandle) error {
	tags := r.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("demodulator") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("demodulator", "rewrite_visit_children", "src/tactic/ufbv/ufbv_rewriter.cpp", 336,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "Detected demodulator cycle: ")
			parentText, e := formulaTraceText(r.manager, app)
			if e != nil {
				operandErr = e
				return e
			}
			_, _ = io.WriteString(w, parentText+" --> ")
			childText, e := formulaTraceText(r.manager, child)
			if e != nil {
				operandErr = e
				return e
			}
			_, _ = io.WriteString(w, childText+"\n")
			return nil
		})
	return operandErr
}

func (r *rewriteTraversal) traceRewrite(index *rewriteRuleIndex, line int, heading string, expr ast.ASTHandle) error {
	tags := index.traceTags()
	if !tags.ShouldTrace("demodulator") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("demodulator", "rewrite", "src/tactic/ufbv/ufbv_rewriter.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, heading)
			body, e := formulaTraceText(r.manager, expr)
			if e != nil {
				operandErr = e
				return e
			}
			_, _ = io.WriteString(w, body+"\n")
			return nil
		})
	return operandErr
}

func (r *rewriteTraversal) traceRebuilt(index *rewriteRuleIndex, original ast.ASTHandle, args []ast.ASTHandle, rebuilt ast.ASTHandle) error {
	tags := index.traceTags()
	if !tags.ShouldTrace("demodulator_bug") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("demodulator_bug", "rewrite", "src/tactic/ufbv/ufbv_rewriter.cpp", 423,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "e:\n")
			oldText, e := formulaTraceText(r.manager, original)
			if e != nil {
				operandErr = e
				return e
			}
			_, _ = io.WriteString(w, oldText+"\nnew_args: \n")
			for _, arg := range args {
				argText, e := formulaTraceText(r.manager, arg)
				if e != nil {
					operandErr = e
					return e
				}
				_, _ = io.WriteString(w, argText+"\n")
			}
			_, _ = io.WriteString(w, "=====>\nna:\n ")
			newText, e := formulaTraceText(r.manager, rebuilt)
			if e != nil {
				operandErr = e
				return e
			}
			_, _ = io.WriteString(w, newText+"\n")
			return nil
		})
	return operandErr
}
