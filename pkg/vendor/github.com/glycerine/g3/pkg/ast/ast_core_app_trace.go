package ast

// Source: src/ast/ast.cpp:2195; retained util/trace TRACE call site.

import (
	"errors"
	"io"
	"strconv"

	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ASTManager) traceChainableApp() {
	tags := m.traceTags
	if tags == nil {
		tags = udiag.BorrowProcessTraceTags()
	}
	_ = tags.EmitConditional("chainable", "mk_app", "src/ast/ast.cpp", 2195,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "chainable...\n")
			return nil
		})
}

func (m *ASTManager) traceAppGround(r astNodeRef) error {
	tags := m.traceTags
	if tags == nil {
		tags = udiag.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("app_ground") {
		return nil
	}
	var bodyErr error
	_ = tags.EmitConditional("app_ground", "mk_app", "src/ast/ast.cpp", 2207,
		func() bool { return true }, func(w io.Writer) error {
			app := m.appAt(r.slot)
			ground := "0"
			if app.ground {
				ground = "1"
			}
			_, _ = io.WriteString(w, "ground: ")
			_, _ = io.WriteString(w, ground)
			_, _ = io.WriteString(w, " id: ")
			_, _ = io.WriteString(w, strconv.FormatUint(uint64(m.nodeAt(r.slot).id), 10))
			_, _ = io.WriteString(w, "\n")
			out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
			if err != nil {
				bodyErr = err
				return err
			}
			printErr := m.lowLevelPrint(out, r, true, true)
			rendered, readErr := out.String()
			_, _ = io.WriteString(w, rendered)
			closeErr := out.Close()
			bodyErr = errors.Join(printErr, readErr, closeErr)
			if bodyErr != nil {
				return bodyErr
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		})
	return bodyErr
}
