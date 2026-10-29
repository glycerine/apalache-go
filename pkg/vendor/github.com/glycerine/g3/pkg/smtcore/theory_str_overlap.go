package smtcore

// Source: src/smt/theory_str.cpp will_result_in_overlap and
// is_concat_eq_type1 through is_concat_eq_type6.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) stringConcatOverlap(left, right ast.ASTHandle) (overlap bool, result error) {
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	first, err := t.simplifyStringConcat(left)
	if err != nil {
		return false, err
	}
	if err := locals.add(first); err != nil {
		return false, err
	}
	second, err := t.simplifyStringConcat(right)
	if err != nil {
		return false, err
	}
	if err := locals.add(second); err != nil {
		return false, err
	}
	m := t.state.manager
	x, y, lc, err := m.SequenceConcatParts(first)
	if err != nil {
		return false, err
	}
	a, b, rc, err := m.SequenceConcatParts(second)
	if err != nil || !lc && !rc {
		return false, err
	}
	// Native accesses both binary applications after the both-nonconcat
	// return. Reached group_terms_by_eqc inputs satisfy this precondition.
	if !lc || !rc {
		return false, ast.ErrASTDomain
	}
	literal := [4]bool{}
	for i, expression := range []ast.ASTHandle{x, y, a, b} {
		if err := t.initializeStringCut(expression); err != nil {
			return false, err
		}
		_, literal[i], err = m.SequenceStringValue(expression)
		if err != nil {
			return false, err
		}
	}
	switch literal {
	case [4]bool{false, false, false, false}:
		return t.hasSelfCut(a, y) || t.hasSelfCut(x, b), nil
	case [4]bool{false, true, false, false}:
		return t.hasSelfCut(x, b), nil
	case [4]bool{false, false, false, true}:
		return t.hasSelfCut(a, y), nil
	case [4]bool{true, false, false, false}:
		return t.hasSelfCut(a, y), nil
	case [4]bool{false, false, true, false}:
		return t.hasSelfCut(x, b), nil
	case [4]bool{true, false, true, false}, [4]bool{false, true, false, true}:
		return false, nil
	case [4]bool{true, false, false, true}:
		return t.hasSelfCut(a, y), nil
	case [4]bool{false, true, true, false}:
		return t.hasSelfCut(x, b), nil
	}
	return false, nil
}
