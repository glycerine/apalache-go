package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:637; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *rewriteBackIndex) traceRescheduling(index *rewriteRuleIndex, quantifier ast.ASTHandle) error {
	tags := index.traceTags()
	if !tags.ShouldTrace("demodulator") {
		return nil
	}
	var operandErr error
	_ = tags.EmitConditional("demodulator", "reschedule_demodulators", "src/tactic/ufbv/ufbv_rewriter.cpp", 637,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "Rescheduling: \n")
			body, e := formulaTraceText(r.manager, quantifier)
			if e != nil {
				operandErr = e
				return e
			}
			_, _ = io.WriteString(w, body+"\n")
			return nil
		})
	return operandErr
}
