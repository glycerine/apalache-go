package ast

// Source: src/ast/ast.cpp:2918-3015; src/util/trace.h:40-46.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func (m *ASTManager) proofResolutionTraceTags() *diagnostic.TraceTags {
	if m.traceTags != nil {
		return m.traceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (m *ASTManager) proofResolutionTraceEnabled(tag string) bool {
	return m.proofResolutionTraceTags().ShouldTrace(tag)
}

func (m *ASTManager) traceUnitResolutionBadClause(first, second ASTHandle) error {
	return m.proofResolutionTraceTags().EmitConditional(
		"mk_unit_resolution_bug", "mk_unit_resolution", "src/ast/ast.cpp", 2918,
		func() bool { return true }, func(w io.Writer) error {
			c, p, globals, err := m.wellSortedTraceParams(nil, nil, nil)
			if err != nil {
				return err
			}
			one, err := m.wellSortedTraceSMT2(first, c, p, globals)
			if err != nil {
				return err
			}
			two, err := m.wellSortedTraceSMT2(second, c, p, globals)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, one+" "+two+"\n")
			return err
		})
}

func (m *ASTManager) traceUnitResolutionFactInput(tag string, line int, proofs []ASTHandle, fact ASTHandle) error {
	return m.proofResolutionTraceTags().EmitConditional(
		tag, "mk_unit_resolution", "src/ast/ast.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			c, p, globals, err := m.wellSortedTraceParams(nil, nil, nil)
			if err != nil {
				return err
			}
			for _, proof := range proofs {
				m.mu.Lock()
				value, proofErr := m.proofFact(proof)
				m.mu.Unlock()
				if proofErr != nil {
					return proofErr
				}
				body, err := m.wellSortedTraceSMT2(value, c, p, globals)
				if err != nil {
					return err
				}
				if _, err = io.WriteString(w, body+"\n"); err != nil {
					return err
				}
			}
			if _, err = io.WriteString(w, "===>\n"); err != nil {
				return err
			}
			body, err := m.wellSortedTraceSMT2(fact, c, p, globals)
			if err != nil {
				return err
			}
			_, err = io.WriteString(w, body+"\n")
			return err
		})
}

func (m *ASTManager) resolutionLowLevel(h ASTHandle) (string, error) {
	out, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: m.BudgetField})
	if err != nil {
		return "", err
	}
	printErr := m.lowLevelPrint(out, astNodeRef{slot: h.slot, generation: h.generation}, true, true)
	body, readErr := out.String()
	closeErr := out.Close()
	return body, errors.Join(printErr, readErr, closeErr)
}

// The caller holds the manager lock for the low-level printer.
func (m *ASTManager) traceUnitResolutionUnmatched(proofs []ASTHandle, missing int) error {
	return m.proofResolutionTraceTags().EmitConditional(
		"mk_unit_resolution_bug", "mk_unit_resolution", "src/ast/ast.cpp", 2942,
		func() bool { return true }, func(w io.Writer) error {
			for j, proof := range proofs {
				if j == missing {
					if _, err := io.WriteString(w, "Index "+strconv.Itoa(missing)+" was not found:\n"); err != nil {
						return err
					}
				}
				fact, err := m.proofFact(proof)
				if err != nil {
					return err
				}
				body, err := m.resolutionLowLevel(fact)
				if err != nil {
					return err
				}
				if _, err = io.WriteString(w, body); err != nil {
					return err
				}
			}
			return nil
		})
}

// The caller holds the manager lock for the low-level printer.
func (m *ASTManager) traceUnitResolutionWrongFact(fact, lit ASTHandle) error {
	return m.proofResolutionTraceTags().EmitConditional(
		"unit_bug1", "mk_unit_resolution", "src/ast/ast.cpp", 3006,
		func() bool { return true }, func(w io.Writer) error {
			for _, h := range [...]ASTHandle{fact, lit} {
				body, err := m.resolutionLowLevel(h)
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
