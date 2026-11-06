package pattern

// Source: src/ast/pattern/pattern_inference.cpp:584,695,704.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (c *patternCandidates) traceReduceQuantifier(q ast.ASTHandle, failed bool) error {
	tag, line := "pattern_inference", 584
	if failed {
		tag, line = "pi_failed", 704
	}
	tags := c.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace(tag) {
		return nil
	}
	return tags.EmitConditional(tag, "reduce_quantifier", "src/ast/pattern/pattern_inference.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			if !failed {
				if _, err := io.WriteString(w, "processing:\n"); err != nil {
					return err
				}
			}
			if err := c.writePatternSMT2(w, q); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
}

func (c *patternCandidates) tracePulledQuantifier(q ast.ASTHandle) error {
	tags := c.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("pattern_inference") {
		return nil
	}
	return tags.EmitConditional("pattern_inference", "reduce_quantifier", "src/ast/pattern/pattern_inference.cpp", 695,
		func() bool { return true }, func(w io.Writer) error {
			if _, err := io.WriteString(w, "pulled quantifier:\n"); err != nil {
				return err
			}
			if err := c.writePatternSMT2(w, q); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\n")
			return err
		})
}
