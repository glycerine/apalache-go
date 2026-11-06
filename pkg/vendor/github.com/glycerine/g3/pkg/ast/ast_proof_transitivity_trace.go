package ast

// Source: src/ast/ast.cpp:2717-2730; src/util/trace.h:40-46.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ASTManager) proofTransitivityTraceTags() *diagnostic.TraceTags {
	if m.traceTags != nil {
		return m.traceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (m *ASTManager) proofTransitivityTraceEnabled() bool {
	return m.proofTransitivityTraceTags().ShouldTrace("mk_transitivity")
}

func (m *ASTManager) traceProofTransitivityFacts(w io.Writer, f, g ASTHandle) error {
	c, p, globals, err := m.wellSortedTraceParams(nil, nil, nil)
	if err != nil {
		return err
	}
	first, err := m.wellSortedTraceSMT2(f, c, p, globals)
	if err != nil {
		return err
	}
	second, err := m.wellSortedTraceSMT2(g, c, p, globals)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, first+"\n\n"+second+"\n")
	return err
}

func (m *ASTManager) traceProofTransitivityDeclarations(f, g, firstDecl, secondDecl ASTHandle) error {
	return m.proofTransitivityTraceTags().EmitConditional(
		"mk_transitivity", "mk_transitivity", "src/ast/ast.cpp", 2717,
		func() bool { return true }, func(w io.Writer) error {
			if err := m.traceProofTransitivityFacts(w, f, g); err != nil {
				return err
			}
			c, p, globals, err := m.wellSortedTraceParams(nil, nil, nil)
			if err != nil {
				return err
			}
			for _, decl := range [...]ASTHandle{firstDecl, secondDecl} {
				format, err := m.SMT2PPDeclareFun(decl)
				if err != nil {
					return err
				}
				body, err := m.wellSortedTraceFormat(format, c, p, globals)
				if err != nil {
					return err
				}
				if _, err = io.WriteString(w, body+"\n"); err != nil {
					return err
				}
			}
			return nil
		})
}

func (m *ASTManager) transitivityBoundedProof(proof ASTHandle) (body string, result error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
	if err != nil {
		return "", err
	}
	defer func() { result = errors.Join(result, out.Close()) }()
	m.mu.Lock()
	printErr := m.boundedLowLevelPrint(out, astNodeRef{slot: proof.slot, generation: proof.generation}, 5)
	m.mu.Unlock()
	if printErr != nil {
		return "", printErr
	}
	return out.String()
}

func (m *ASTManager) traceProofTransitivityMiddle(f, g, p, q ASTHandle) error {
	return m.proofTransitivityTraceTags().EmitConditional(
		"mk_transitivity", "mk_transitivity", "src/ast/ast.cpp", 2726,
		func() bool { return true }, func(w io.Writer) error {
			if err := m.traceProofTransitivityFacts(w, f, g); err != nil {
				return err
			}
			for _, proof := range [...]ASTHandle{p, q} {
				body, err := m.transitivityBoundedProof(proof)
				if err != nil {
					return err
				}
				if _, err = io.WriteString(w, body+"\n\n"); err != nil {
					return err
				}
			}
			return nil
		})
}
