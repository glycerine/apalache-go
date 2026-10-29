package ast

// Source: src/ast/rewriter/fpa_rewriter.cpp:765,769; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

func (m *ASTManager) emitFPAToIEEEBVTrace(line int, body func(io.Writer) error) error {
	tags := m.traceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("fp_rewriter") {
		return nil
	}
	err := tags.EmitConditional("fp_rewriter", "mk_to_ieee_bv", "src/ast/rewriter/fpa_rewriter.cpp", line,
		func() bool { return true }, body)
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

// Called without the AST manager lock: SMT2PPExprTree and PrintFormat acquire
// it as the source printer traverses the concrete argument.
func (m *ASTManager) traceFPAToIEEEBVArgument(h ASTHandle) error {
	return m.emitFPAToIEEEBVTrace(765, func(w io.Writer) error {
		writeFPATracePart(w, "to_ieee_bv of ")
		globals := uparams.BorrowProcessGlobalParamsService()
		if globals == nil || globals.Context == nil {
			return uparams.ErrGlobalParamsOwner
		}
		empty := globals.Context.EmptyValue()
		body, err := m.wellSortedTraceSMT2(h, globals.Context, &empty, globals)
		if err != nil {
			return err
		}
		writeFPATracePart(w, body)
		endFPATraceLine(w)
		return nil
	})
}

func (m *ASTManager) traceFPAToIEEEBVNumeral(v *unumeral.ArbitraryFloat) error {
	return m.emitFPAToIEEEBVTrace(769, func(w io.Writer) error {
		writeFPATracePart(w, "to_ieee_bv numeral: ")
		body, err := m.floatingPointPlugin.numeric.ToString(v)
		if err != nil {
			return err
		}
		writeFPATracePart(w, body)
		endFPATraceLine(w)
		return nil
	})
}
