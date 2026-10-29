package pattern

// Source: src/ast/pattern/pattern_inference.cpp:531-565.

import (
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// tracePatternStage displays the concrete candidate slices at the source
// mk_patterns checkpoints. The initial checkpoint also displays its body.
func (c *patternCandidates) tracePatternStage(line int, body ast.ASTHandle, candidates []ast.ASTHandle) error {
	tags := c.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("pattern_inference") {
		return nil
	}
	return tags.EmitConditional("pattern_inference", "mk_patterns", "src/ast/pattern/pattern_inference.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			switch line {
			case 531:
				if err := c.writePatternSMT2(w, body); err != nil {
					return err
				}
				if _, err := io.WriteString(w, "\ncandidates:\n"); err != nil {
					return err
				}
			case 542:
				if _, err := io.WriteString(w, "candidates after removing looping-patterns:\n"); err != nil {
					return err
				}
			case 551:
				if _, err := io.WriteString(w, "candidates after removing bigger patterns:\n"); err != nil {
					return err
				}
			case 562:
				if _, err := io.WriteString(w, "candidates after sorting:\n"); err != nil {
					return err
				}
			default:
				return ast.ErrASTDomain
			}
			for _, candidate := range candidates {
				if err := c.writePatternSMT2(w, candidate); err != nil {
					return err
				}
				if _, err := io.WriteString(w, "\n"); err != nil {
					return err
				}
			}
			return nil
		})
}
