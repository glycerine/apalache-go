package ast

// Source: src/ast/ast.cpp:3026,3102; src/util/trace.h:40-46.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ASTManager) proofRuleTraceTags() *diagnostic.TraceTags {
	if m.traceTags != nil {
		return m.traceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (m *ASTManager) proofRuleTraceEnabled(tag string) bool {
	return m.proofRuleTraceTags().ShouldTrace(tag)
}

// The caller holds the manager lock, as the low-level proof printer expects.
func (m *ASTManager) traceProofLemma(p ASTHandle) error {
	return m.proofRuleTraceTags().EmitConditional(
		"mk_lemma", "mk_lemma", "src/ast/ast.cpp", 3026,
		func() bool { return true }, func(w io.Writer) error {
			out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
			if err != nil {
				return err
			}
			printErr := m.lowLevelPrint(out, astNodeRef{slot: p.slot, generation: p.generation}, true, true)
			body, readErr := out.String()
			_, writeErr := io.WriteString(w, body+"\n")
			closeErr := out.Close()
			return errors.Join(printErr, readErr, writeErr, closeErr)
		})
}

// The SMT2 printer acquires the manager lock, so the caller releases it first.
func (m *ASTManager) traceProofAndElim(f ASTHandle, i uint32) error {
	return m.proofRuleTraceTags().EmitConditional(
		"mk_and_elim", "mk_and_elim", "src/ast/ast.cpp", 3102,
		func() bool { return true }, func(w io.Writer) error {
			c, p, globals, err := m.wellSortedTraceParams(nil, nil, nil)
			if err != nil {
				return err
			}
			body, err := m.wellSortedTraceSMT2(f, c, p, globals)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, "i: "+strconv.FormatUint(uint64(i), 10)+"\n"+body+"\n")
			return err
		})
}
