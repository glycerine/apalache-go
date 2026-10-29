package ast

// Source: src/ast/ast.cpp:2964,3015; src/util/trace.h.

import (
	"errors"
	"io"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ASTManager) traceUnitResolution(proof ASTHandle, line int, label string) error {
	tags := m.traceTags
	if tags == nil {
		tags = udiag.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("unit_resolution") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("unit_resolution", "mk_unit_resolution", "src/ast/ast.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, label)
			out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
			if err != nil {
				bodyErr = err
				return err
			}
			printErr := m.lowLevelPrint(out, astNodeRef{slot: proof.slot, generation: proof.generation}, true, true)
			rendered, readErr := out.String()
			_, _ = io.WriteString(w, rendered)
			closeErr := out.Close()
			bodyErr = errors.Join(printErr, readErr, closeErr)
			return bodyErr
		})
	return bodyErr
}
