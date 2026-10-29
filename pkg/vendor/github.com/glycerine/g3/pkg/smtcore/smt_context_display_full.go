package smtcore

// Source: src/smt/smt_context_pp.cpp context::display (296-326).

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
)

func smtDisplayBool(value bool) int {
	if value {
		return 1
	}
	return 0
}

// DisplayContext composes the native logical-context dump in source order.
// The search base is the native base level; assumptions advance the native
// search level to the retained search base while ordinary decisions advance
// only scopeLevel.
func (s *SMTInternalizerState) DisplayContext(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	out = smtContextDisplayWriter{out}
	baseLevel, searchLevel := s.scopeLevel, s.scopeLevel
	if s.searching {
		baseLevel = s.searchUserBaseLevel
		searchLevel = s.searchBaseLevel
	}
	assertedInconsistent := false
	if s.assertedInconsistent != nil {
		assertedInconsistent = *s.assertedInconsistent
	}
	if err := smtWriteTheoryText(out, fmt.Sprintf("Logical context:\nscope-lvl: %d\nbase-lvl:  %d\nsearch-lvl:  %d\ninconsistent(): %d\nm_asserted_formulas.inconsistent(): %d\n",
		s.scopeLevel, baseLevel, searchLevel, smtDisplayBool(s.inconsistent), smtDisplayBool(assertedInconsistent))); err != nil {
		return err
	}
	steps := []func(io.Writer) error{
		s.DisplayDefinitions,
		s.DisplayClauseSections,
		s.DisplayBinaryClauses,
		s.DisplayAssignment,
		s.DisplayEquivalenceClasses,
		func(io.Writer) error { return s.congruence.DisplayCompact() },
		func(writer io.Writer) error { return s.displayCaseSplitQueue(writer, s.caseSplitStrategy) },
		s.DisplayExpressionBoolVarMap,
		s.DisplayApplicationEnodeMap,
		s.DisplayRelevantExpressions,
		s.DisplayTheories,
		s.DisplayDeclarationEnodes,
		s.DisplayHotBooleanVariables,
	}
	for _, step := range steps {
		if err := step(out); err != nil {
			return err
		}
	}
	return nil
}
