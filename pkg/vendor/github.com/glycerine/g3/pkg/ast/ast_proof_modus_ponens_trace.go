package ast

// Source: src/ast/ast.cpp:2641-2649; src/util/trace.h:40-46.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ASTManager) proofModusPonensTraceTags() *diagnostic.TraceTags {
	if m.traceTags != nil {
		return m.traceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (m *ASTManager) proofModusPonensTraceEnabled() bool {
	return m.proofModusPonensTraceTags().ShouldTrace("mk_modus_ponens")
}

func (m *ASTManager) traceProofModusPonensBadFact(p, q ASTHandle) error {
	return m.proofModusPonensTraceTags().EmitConditional(
		"mk_modus_ponens", "mk_modus_ponens", "src/ast/ast.cpp", 2641,
		func() bool { return true }, func(w io.Writer) error {
			for _, proof := range [...]ASTHandle{p, q} {
				out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
				if err != nil {
					return err
				}
				printErr := m.lowLevelPrint(out, astNodeRef{slot: proof.slot, generation: proof.generation}, true, true)
				body, readErr := out.String()
				_, writeErr := io.WriteString(w, body+"\n")
				closeErr := out.Close()
				if err = errors.Join(printErr, readErr, writeErr, closeErr); err != nil {
					return err
				}
			}
			return nil
		})
}

func (m *ASTManager) traceProofModusPonensSMT2(line int, prefix string, values ...ASTHandle) error {
	return m.proofModusPonensTraceTags().EmitConditional(
		"mk_modus_ponens", "mk_modus_ponens", "src/ast/ast.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			c, p, g, err := m.wellSortedTraceParams(nil, nil, nil)
			if err != nil {
				return err
			}
			for _, value := range values {
				body, err := m.wellSortedTraceSMT2(value, c, p, g)
				if err != nil {
					return err
				}
				if _, err = io.WriteString(w, prefix+body+"\n"); err != nil {
					return err
				}
			}
			return nil
		})
}

func (m *ASTManager) proofModusPonensNonground(h ASTHandle) (bool, error) {
	r, err := m.requireExpr(h)
	if err != nil {
		return false, err
	}
	if m.nodeAt(r.slot).kind != ASTApp {
		return false, nil
	}
	a := m.appAt(r.slot)
	return !a.ground && !a.quantifiers, nil
}
