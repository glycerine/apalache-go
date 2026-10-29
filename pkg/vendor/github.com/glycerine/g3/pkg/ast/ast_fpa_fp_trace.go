package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:700; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (m *ASTManager) traceFPAFPValue(v *numeral.ArbitraryFloat) error {
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("fp_rewriter") {
		return nil
	}
	err := tags.EmitConditional("fp_rewriter", "mk_fp", "src/ast/rewriter/fpa_rewriter.cpp", 700,
		func() bool { return true }, func(w io.Writer) error {
			writeFPATracePart(w, "simplified (fp ...) to ")
			value, e := m.floatingPointPlugin.numeric.ToString(v)
			if e != nil {
				return e
			}
			writeFPATracePart(w, value)
			endFPATraceLine(w)
			return nil
		})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
