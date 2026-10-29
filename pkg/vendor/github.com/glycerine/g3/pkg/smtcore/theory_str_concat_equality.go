package smtcore

// Source: src/smt/theory_str.cpp infer_len_concat_arg,
// infer_len_concat_equality and the active simplify_concat_equality dispatch.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) inferStringConcatArgument(concat ast.ASTHandle, total *big.Int) (result error) {
	if total.Sign() < 0 {
		return nil
	}
	m := t.state.manager
	first, second, isConcat, err := m.SequenceConcatParts(concat)
	if err != nil || !isConcat {
		return err
	}
	left, lk, err := t.lengthValue(first)
	if err != nil {
		return err
	}
	right, rk, err := t.lengthValue(second)
	if err != nil || lk == rk {
		return err
	}
	missing, known, value := first, second, right
	if lk {
		missing, known, value = second, first, left
	}
	difference := new(big.Int).Sub(total, value)
	if difference.Sign() < 0 {
		return nil
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	if err := premises.length(concat, total); err != nil {
		return err
	}
	if err := premises.length(known, value); err != nil {
		return err
	}
	conclusion := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, conclusion.close()) }()
	if err := conclusion.length(missing, difference); err != nil {
		return err
	}
	return premises.imply(conclusion.values[0])
}

func (t *SMTStringTheory) inferStringConcatEquality(left, right ast.ASTHandle) error {
	total, known, err := t.lengthValue(left)
	if err != nil {
		return err
	}
	if !known {
		total, known, err = t.lengthValue(right)
		if err != nil {
			return err
		}
	}
	// Native infer_len_concat's assertion branch is unreachable with the
	// recursive get_len_value: known children imply a known concat sum.
	if !known {
		return nil
	}
	if err := t.inferStringConcatArgument(left, total); err != nil {
		return err
	}
	return t.inferStringConcatArgument(right, total)
}

// Native shared-concat groups skip step 1 entirely. Otherwise select the
// first nonoverlapping pair, falling back to the first pair if all overlap.
func (t *SMTStringTheory) simplifyStringConcatGroups(left, right smtStringClassGroups) error {
	for _, l := range left.concats {
		for _, r := range right.concats {
			if l == r {
				return nil
			}
		}
	}
	if len(left.concats) == 0 || len(right.concats) == 0 {
		return nil
	}
	for _, l := range left.concats {
		for _, r := range right.concats {
			overlap, err := t.stringConcatOverlap(l, r)
			if err != nil {
				return err
			}
			if !overlap {
				return t.simplifyStringConcatEquality(l, r)
			}
		}
	}
	return t.simplifyStringConcatEquality(left.concats[0], right.concats[0])
}

func (t *SMTStringTheory) simplifyStringConcatEquality(left, right ast.ASTHandle) (result error) {
	m := t.state.manager
	x, y, lc, err := m.SequenceConcatParts(left)
	if err != nil {
		return err
	}
	a, b, rc, err := m.SequenceConcatParts(right)
	if err != nil || !lc || !rc {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	// Snapshot all four lengths before inference, as in the source.
	lengths := [4]*big.Int{}
	known := [4]bool{}
	for i, expression := range []ast.ASTHandle{x, y, a, b} {
		lengths[i], known[i], err = t.lengthValue(expression)
		if err != nil {
			return err
		}
	}
	if err := t.inferStringConcatEquality(left, right); err != nil {
		return err
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	if err := premises.equate(left, right); err != nil {
		return err
	}
	conclusions := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, conclusions.close()) }()
	temporaries := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, temporaries.close()) }()
	lengthEquality := func(first, second ast.ASTHandle, target *smtStringLengthPremises) error {
		l, err := m.MkSequenceLength(first)
		if err != nil {
			return err
		}
		if err := temporaries.add(l); err != nil {
			return err
		}
		r, err := m.MkSequenceLength(second)
		if err != nil {
			return err
		}
		if err := temporaries.add(r); err != nil {
			return err
		}
		return target.equate(l, r)
	}
	cancel := func(first, second ast.ASTHandle) error {
		if err := conclusions.equate(first, second); err != nil {
			return err
		}
		if err := lengthEquality(first, second, &conclusions); err != nil {
			return err
		}
		conclusion, err := m.MkAndArray(conclusions.values)
		if err != nil {
			return err
		}
		return premises.imply(conclusion)
	}
	if x == a {
		same, err := t.sameStringClass(y, b)
		if err != nil || same {
			return err
		}
		return cancel(y, b)
	}
	if y == b {
		same, err := t.sameStringClass(x, a)
		if err != nil || same {
			return err
		}
		return cancel(x, a)
	}
	for i, pair := range [][4]ast.ASTHandle{{x, a, y, b}, {y, b, x, a}} {
		same, err := t.sameStringClass(pair[0], pair[1])
		if err != nil {
			return err
		}
		if same {
			if i == 0 {
				otherSame, err := t.sameStringClass(pair[2], pair[3])
				if err != nil || otherSame {
					return err
				}
			}
			if err := premises.equate(pair[0], pair[1]); err != nil {
				return err
			}
			return cancel(pair[2], pair[3])
		}
	}
	for i, pair := range [][2]ast.ASTHandle{{x, a}, {y, b}} {
		if !known[i] || !known[i+2] || lengths[i].Cmp(lengths[i+2]) != 0 {
			continue
		}
		if err := lengthEquality(pair[0], pair[1], &premises); err != nil {
			return err
		}
		if err := conclusions.equate(x, a); err != nil {
			return err
		}
		if err := conclusions.equate(y, b); err != nil {
			return err
		}
		conclusion, err := m.MkAndArray(conclusions.values)
		if err != nil {
			return err
		}
		return premises.imply(conclusion)
	}
	newLeft, err := t.simplifyStringConcat(left)
	if err != nil {
		return err
	}
	if err := temporaries.add(newLeft); err != nil {
		return err
	}
	newRight, err := t.simplifyStringConcat(right)
	if err != nil || newLeft == newRight {
		return err
	}
	if err := temporaries.add(newRight); err != nil {
		return err
	}
	possible, err := t.canTwoNodesEqual(newLeft, newRight)
	if err != nil {
		return err
	}
	if !possible {
		equality, err := m.MkEq(newLeft, newRight)
		if err != nil {
			return err
		}
		if err := temporaries.add(equality); err != nil {
			return err
		}
		negative, err := m.MkNot(equality)
		if err != nil {
			return err
		}
		return t.assertAxiom(negative)
	}
	x, y, lc, err = m.SequenceConcatParts(newLeft)
	if err != nil {
		return err
	}
	a, b, rc, err = m.SequenceConcatParts(newRight)
	if err != nil {
		return err
	}
	if !lc || !rc {
		if _, literal, err := m.SequenceStringValue(newLeft); err != nil {
			return err
		} else if literal && rc {
			return t.simplifyStringParents(newRight, newLeft)
		}
		if _, literal, err := m.SequenceStringValue(newRight); err != nil {
			return err
		} else if literal && lc {
			return t.simplifyStringParents(newLeft, newRight)
		}
		return nil
	}
	same, err := t.sameStringClass(newLeft, newRight)
	if err != nil {
		return err
	}
	if !same && (left != newLeft || right != newRight) {
		guards := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, guards.close()) }()
		for _, pair := range [][2]ast.ASTHandle{{left, newLeft}, {right, newRight}, {left, right}} {
			if err := guards.equate(pair[0], pair[1]); err != nil {
				return err
			}
		}
		equality, err := m.MkEq(newLeft, newRight)
		if err != nil {
			return err
		}
		if err := guards.imply(equality); err != nil {
			return err
		}
	}
	allVariables := true
	literals := [4]bool{}
	for i, expression := range []ast.ASTHandle{x, y, a, b} {
		if err := t.initializeStringCut(expression); err != nil {
			return err
		}
		_, literal, err := m.SequenceStringValue(expression)
		if err != nil {
			return err
		}
		allVariables = allVariables && !literal
		literals[i] = literal
	}
	if allVariables {
		return t.arrangeVariableConcats(newLeft, newRight)
	}
	if literals == [4]bool{false, false, false, true} || literals == [4]bool{false, true, false, false} {
		return t.arrangeLiteralSuffixConcats(newLeft, newRight)
	}
	if literals == [4]bool{true, false, false, false} || literals == [4]bool{false, false, true, false} {
		return t.arrangeLiteralPrefixConcats(newLeft, newRight)
	}
	if literals == [4]bool{true, false, true, false} {
		return t.arrangeMatchingLiteralBoundaries(newLeft, newRight, false)
	}
	if literals == [4]bool{false, true, false, true} {
		return t.arrangeMatchingLiteralBoundaries(newLeft, newRight, true)
	}
	if literals == [4]bool{true, false, false, true} || literals == [4]bool{false, true, true, false} {
		return t.arrangeOpposingLiteralBoundaries(newLeft, newRight)
	}
	return nil
}
