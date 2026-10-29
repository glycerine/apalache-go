package ast

// Source: src/ast/ast.cpp:3152,3167; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *ASTManager) proofHyperTraceTags() *diagnostic.TraceTags {
	if m.traceTags != nil {
		return m.traceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (m *ASTManager) proofHyperTraceEnabled() bool {
	return m.proofHyperTraceTags().ShouldTrace("hyper_res")
}

// The SMT2 printer acquires the manager lock, so the caller releases it first.
func (m *ASTManager) traceProofHyperPremise(premise ASTHandle) error {
	return m.proofHyperTraceTags().EmitConditional(
		"hyper_res", "mk_hyper_resolve", "src/ast/ast.cpp", 3152,
		func() bool { return true }, func(w io.Writer) error {
			c, p, globals, err := m.wellSortedTraceParams(nil, nil, nil)
			if err != nil {
				return err
			}
			body, err := m.wellSortedTraceSMT2(premise, c, p, globals)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, body+"\n")
			return err
		})
}

// Parameter display strings are captured while the caller holds the manager
// lock; source parameter::display writes AST IDs and signed position integers.
func (m *ASTManager) traceProofHyperParameters(parameters []string) error {
	return m.proofHyperTraceTags().EmitConditional(
		"hyper_res", "mk_hyper_resolve", "src/ast/ast.cpp", 3167,
		func() bool { return true }, func(w io.Writer) error {
			for _, parameter := range parameters {
				if _, err := io.WriteString(w, parameter+"\n"); err != nil {
					return err
				}
			}
			return nil
		})
}
