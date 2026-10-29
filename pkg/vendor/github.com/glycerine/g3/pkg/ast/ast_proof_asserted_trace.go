package ast

// Source: src/ast/ast.cpp:2627; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *ASTManager) proofAssertedTraceTags() *diagnostic.TraceTags {
	if m.traceTags != nil {
		return m.traceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (m *ASTManager) proofAssertedTraceEnabled() bool {
	return m.proofAssertedTraceTags().ShouldTrace("mk_asserted_bug")
}

func (m *ASTManager) traceProofAssertedNonBool(h ASTHandle) error {
	return m.proofAssertedTraceTags().EmitConditional(
		"mk_asserted_bug", "mk_asserted", "src/ast/ast.cpp", 2627,
		func() bool { return true }, func(w io.Writer) error {
			c, p, g, err := m.wellSortedTraceParams(nil, nil, nil)
			if err != nil {
				return err
			}
			body, err := m.wellSortedTraceSMT2(h, c, p, g)
			if err != nil {
				return err
			}
			if _, err = io.WriteString(w, body+"\nsort: "); err != nil {
				return err
			}
			sort, err := m.GetSort(h)
			if err != nil {
				return err
			}
			format, err := m.SMT2PPSort(sort)
			if err != nil {
				return err
			}
			body, err = m.wellSortedTraceFormat(format, c, p, g)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, body+"\n")
			return err
		})
}
