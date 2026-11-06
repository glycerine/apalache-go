package smtcore

// Source: src/smt/theory_seq.cpp check_length_coherence, mk_decompose and
// propagate_length_coherence.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTSequenceTheory) checkTrackedSequenceLengthCoherence() (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	var roots []ast.ASTHandle
	for _, node := range t.variables {
		if node == nil {
			continue
		}
		expression := node.Owner()
		if _, tracked := t.lengths[expression]; !tracked || !t.solutions.IsRoot(expression) {
			continue
		}
		variable, err := t.sequenceVariable(expression)
		if err != nil {
			return false, err
		}
		if variable {
			roots = append(roots, expression)
		}
	}
	// Source check_length_coherence0 tries the empty branch for every
	// tracked solution root before exposing any new head/tail decomposition.
	for _, expression := range roots {
		sort, err := state.manager.GetSort(expression)
		if err != nil {
			return false, err
		}
		empty, err := state.manager.MkSequenceEmpty(sort)
		if err != nil {
			return false, err
		}
		value, err := t.assumeSequenceEquality(expression, empty)
		if err != nil {
			return false, err
		}
		if value != lbool.False {
			return true, nil
		}
	}
	for _, expression := range roots {
		before := state.searchProgress()
		if err = t.addSequenceCoherenceAxiom(expression); err != nil {
			return false, err
		}
		sort, err := state.manager.GetSort(expression)
		if err != nil {
			return false, err
		}
		zero, err := t.integer(0)
		if err != nil {
			return false, err
		}
		tail, err := t.relationSkolem("seq.tail", []ast.ASTHandle{expression, zero}, sort)
		if err != nil {
			return false, err
		}
		empty, err := state.manager.MkSequenceEmpty(sort)
		if err != nil {
			return false, err
		}
		if _, err = t.assumeSequenceEquality(tail, empty); err != nil {
			return false, err
		}
		if t.axioms.Pending() || state.searchProgress() != before {
			return true, nil
		}
	}
	return false, nil
}

func (t *SMTSequenceTheory) trackInternalTerms(expressions ...ast.ASTHandle) {
	for _, expression := range expressions {
		if _, found := t.internalTerms[expression]; !found {
			t.internalTerms[expression] = struct{}{}
			t.internalLog = append(t.internalLog, expression)
		}
	}
}

// addSequenceCoherenceAxiom gives every symbolic sequence its source-level
// empty-or-cons decomposition.  The two Skolems are stable ASTs, so later
// arithmetic bounds and word-equation rules can refer to the same head and
// tail witnesses.
func (t *SMTSequenceTheory) addSequenceCoherenceAxiom(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	sort, err := state.manager.GetSort(expression)
	if err != nil {
		return err
	}
	elementSort, sequence, err := state.manager.SequenceSortElement(sort)
	if err != nil {
		return err
	}
	if !sequence {
		return ast.ErrASTDomain
	}
	empty, err := state.manager.MkSequenceEmpty(sort)
	if err != nil {
		return err
	}
	emptyEquality, err := state.manager.MkEq(expression, empty)
	if err != nil {
		return err
	}
	zero, err := t.integer(0)
	if err != nil {
		return err
	}
	head, err := t.relationSkolem("seq.nth", []ast.ASTHandle{expression, zero}, elementSort)
	if err != nil {
		return err
	}
	tail, err := t.relationSkolem("seq.tail", []ast.ASTHandle{expression, zero}, sort)
	if err != nil {
		return err
	}
	unit, err := state.manager.MkSequenceUnit(head)
	if err != nil {
		return err
	}
	word, err := state.manager.MkSequenceConcat(unit, tail)
	if err != nil {
		return err
	}
	decomposition, err := state.manager.MkEq(expression, word)
	if err != nil {
		return err
	}
	coherence, err := state.manager.MkOr(emptyEquality, decomposition)
	if err != nil {
		return err
	}
	t.trackInternalTerms(head, tail, unit, word)
	return t.axioms.Push(coherence)
}
