package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:241,367. The source prints raw
// C++ node pointers; Go nodes have stable typed origin/slot/generation handles.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func traceASTIdentity(h ast.ASTHandle) string {
	return "@" + strconv.FormatUint(h.Origin(), 16) + ":" +
		strconv.FormatUint(uint64(h.Slot()), 16) + ":" +
		strconv.FormatUint(uint64(h.Generation()), 16)
}

func (r *rewriteRuleIndex) traceRemove(quantifier ast.ASTHandle) {
	tags := r.traceTags()
	if !tags.ShouldTrace("demodulator_fwd") {
		return
	}
	_ = tags.EmitConditional("demodulator_fwd", "remove_fwd_idx", "src/tactic/ufbv/ufbv_rewriter.cpp", 241,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "REMOVE: "+traceASTIdentity(quantifier)+"\n")
			return nil
		})
}

func (r *rewriteTraversal) traceStack() error {
	tags := r.trace
	if tags == nil {
		return nil
	}
	if !tags.ShouldTrace("demodulator_stack") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("demodulator_stack", "rewrite", "src/tactic/ufbv/ufbv_rewriter.cpp", 367,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "STACK: \n")
			for i, h := range r.todo {
				_, _ = io.WriteString(w, strconv.Itoa(i)+": "+traceASTIdentity(h)+" = ")
				body, e := formulaTraceText(r.manager, h)
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
