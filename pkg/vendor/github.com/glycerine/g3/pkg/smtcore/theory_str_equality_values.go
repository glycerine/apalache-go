package smtcore

// Source: src/smt/theory_str.cpp can_concat_eq_str,
// can_concat_eq_concat, can_two_nodes_eq and private EQC traversal.

import (
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) concatLeaves(expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	work := []ast.ASTHandle{expression}
	var leaves []ast.ASTHandle
	for len(work) != 0 {
		last := len(work) - 1
		current := work[last]
		work = work[:last]
		left, right, concat, err := t.state.manager.SequenceConcatParts(current)
		if err != nil {
			return nil, err
		}
		if concat {
			work = append(work, right, left)
		} else {
			// get_nodes_in_concat retains empty-string leaves as well.
			leaves = append(leaves, current)
		}
	}
	return leaves, nil
}

func (t *SMTStringTheory) canConcatEqualString(expression ast.ASTHandle, text string) (bool, error) {
	leaves, err := t.concatLeaves(expression)
	if err != nil {
		return false, err
	}
	m := t.state.manager
	first, literal, err := m.SequenceStringValue(leaves[0])
	if err != nil || literal && !strings.HasPrefix(text, first) {
		return false, err
	}
	last, literal, err := m.SequenceStringValue(leaves[len(leaves)-1])
	if err != nil || literal && !strings.HasSuffix(text, last) {
		return false, err
	}
	var length uint32
	for _, leaf := range leaves {
		part, literal, err := m.SequenceStringValue(leaf)
		if err != nil {
			return false, err
		}
		if literal {
			if !strings.Contains(text, part) {
				return false, nil
			}
			length += uint32(len(part))
		}
	}
	return length <= uint32(len(text)), nil
}

func (t *SMTStringTheory) canConcatsEqual(left, right ast.ASTHandle) (bool, error) {
	l, err := t.concatLeaves(left)
	if err != nil {
		return false, err
	}
	r, err := t.concatLeaves(right)
	if err != nil {
		return false, err
	}
	m := t.state.manager
	for _, suffix := range []bool{false, true} {
		li, ri := 0, 0
		if suffix {
			li, ri = len(l)-1, len(r)-1
		}
		ls, lc, err := m.SequenceStringValue(l[li])
		if err != nil {
			return false, err
		}
		rs, rc, err := m.SequenceStringValue(r[ri])
		if err != nil {
			return false, err
		}
		if !lc || !rc {
			continue
		}
		count := len(ls)
		if len(rs) < count {
			count = len(rs)
		}
		if suffix {
			ls, rs = ls[len(ls)-count:], rs[len(rs)-count:]
		} else {
			ls, rs = ls[:count], rs[:count]
		}
		if ls != rs {
			return false, nil
		}
	}
	return true, nil
}

func (t *SMTStringTheory) canTwoNodesEqual(left, right ast.ASTHandle) (bool, error) {
	m := t.state.manager
	ls, lc, err := m.SequenceStringValue(left)
	if err != nil {
		return false, err
	}
	rs, rc, err := m.SequenceStringValue(right)
	if err != nil {
		return false, err
	}
	if lc && rc {
		return ls == rs, nil
	}
	lcct, err := m.IsSequenceConcat(left)
	if err != nil {
		return false, err
	}
	rcct, err := m.IsSequenceConcat(right)
	if err != nil {
		return false, err
	}
	if lcct && rc {
		return t.canConcatEqualString(left, rs)
	}
	if rcct && lc {
		return t.canConcatEqualString(right, ls)
	}
	if lcct && rcct {
		return t.canConcatsEqual(left, right)
	}
	return true, nil
}

func (t *SMTStringTheory) stringClass(expression ast.ASTHandle) []ast.ASTHandle {
	node := t.state.context.enodes[expression]
	if node == nil || !t.IsAttachedToVariable(node) {
		return []ast.ASTHandle{expression}
	}
	first := uint32(node.TheoryVar(t.FamilyID()))
	var values []ast.ASTHandle
	for current := first; ; {
		values = append(values, t.variables[current].Owner())
		current = t.find.Next(current)
		if current == first {
			return values
		}
	}
}

// The value/length pair loop from new_eq_check runs while the two private
// classes are still separate; handle_equality performs the later stages.
func (t *SMTStringTheory) checkNewEqualityValuesAndLengths(left, right ast.ASTHandle) (bool, error) {
	m := t.state.manager
	for _, l := range t.stringClass(left) {
		for _, r := range t.stringClass(right) {
			possible, err := t.canTwoNodesEqual(l, r)
			if err != nil {
				return false, err
			}
			if !possible {
				equality, err := m.MkEq(l, r)
				if err != nil {
					return false, err
				}
				negative, err := m.MkNot(equality)
				if err != nil {
					return false, err
				}
				return false, t.assertAxiom(negative)
			}
			consistent, err := t.checkLengthConsistency(l, r)
			if err != nil || !consistent {
				return false, err
			}
		}
	}
	if err := t.checkContainNewEquality(left, right); err != nil {
		return false, err
	}
	if len(t.regexMemberships) != 0 {
		automata, err := t.regexAutomataEnabled()
		if err != nil {
			return false, err
		}
		if !automata {
			if err := t.checkRegexNewEquality(left, right); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}
