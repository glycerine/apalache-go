package rewriter

// Source: src/ast/rewriter/seq_rewriter.cpp:1572-1931.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type sequenceEqualityReduction struct {
	rewriter                    *SequenceRewriter
	refs                        []ast.ASTRef
	left                        []ast.ASTHandle
	right                       []ast.ASTHandle
	parts                       *SequenceEqualityReduction
	residualLeft, residualRight []ast.ASTHandle
}

// SequenceEqualityReduction owns the temporary ASTs used by reduce_eq.
// Left/Right are residual word vectors; EqualitiesLeft/Right are its ordered
// side equations, which can have sequence, regex, or element sorts.
type SequenceEqualityReduction struct {
	Left, Right                     []ast.ASTHandle
	EqualitiesLeft, EqualitiesRight []ast.ASTHandle
	Changed, Inconsistent           bool
	manager                         *ast.ASTManager
	refs                            []ast.ASTRef
}

func (r *SequenceEqualityReduction) Close() (err error) {
	if r == nil || r.manager == nil {
		return ast.ErrASTHandle
	}
	for i := len(r.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, r.manager.DelRef(&r.refs[i]))
	}
	r.refs = nil
	r.Left, r.Right, r.EqualitiesLeft, r.EqualitiesRight = nil, nil, nil, nil
	r.manager = nil
	return err
}

// ReduceEquality exposes the live theory_seq reduce_eq consumer without
// packaging residual words and element equations into one Boolean formula.
func (r *SequenceRewriter) ReduceEquality(left, right ast.ASTHandle) (*SequenceEqualityReduction, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	parts := &SequenceEqualityReduction{manager: r.manager}
	_, err := r.rewriteEquality(left, right, ast.ASTRef{}, parts)
	if err != nil {
		return nil, errors.Join(err, parts.Close())
	}
	return parts, nil
}

func (s *sequenceEqualityReduction) close() (err error) {
	for i := len(s.refs) - 1; i >= 0; i-- {
		err = errors.Join(err, s.rewriter.manager.DelRef(&s.refs[i]))
	}
	return err
}

func (s *sequenceEqualityReduction) hold(h ast.ASTHandle) error {
	ref, err := s.rewriter.manager.NewRef(h)
	if err != nil {
		return err
	}
	s.refs = append(s.refs, ref)
	return nil
}

func (s *sequenceEqualityReduction) add(left, right ast.ASTHandle) {
	s.left = append(s.left, left)
	s.right = append(s.right, right)
}

func (s *sequenceEqualityReduction) stringValue(text string) (ast.ASTHandle, error) {
	h, err := s.rewriter.manager.MkString(text)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.hold(h); err != nil {
		return ast.ASTHandle{}, err
	}
	return h, nil
}

func (s *sequenceEqualityReduction) character(value byte) (ast.ASTHandle, error) {
	h, err := s.rewriter.manager.BitVectorNumeralUnsigned(uint32(value), 8)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = s.hold(h); err != nil {
		return ast.ASTHandle{}, err
	}
	return h, nil
}

func (s *sequenceEqualityReduction) concat(parts []ast.ASTHandle, like ast.ASTHandle) (ast.ASTHandle, error) {
	ref, h, err := s.rewriter.makeSequencePartsRef(parts, like)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	s.refs = append(s.refs, ref)
	return h, nil
}

func (s *sequenceEqualityReduction) concreteString(parts []ast.ASTHandle) (string, bool, error) {
	result := ""
	for _, part := range parts {
		text, stringValue, err := s.rewriter.manager.SequenceStringValue(part)
		if err != nil {
			return "", false, err
		}
		if stringValue {
			result += text
			continue
		}
		element, unit, err := s.rewriter.manager.SequenceUnitArgument(part)
		if err != nil {
			return "", false, err
		}
		if !unit {
			return "", false, nil
		}
		value, numeral, err := s.rewriter.manager.SequenceUnitByteElementValue(element)
		if err != nil {
			return "", false, err
		}
		if !numeral {
			return "", false, nil
		}
		result += string([]byte{value})
	}
	return result, true, nil
}

func (s *sequenceEqualityReduction) solveIntToString(left, right []ast.ASTHandle) (bool, error) {
	if len(left) == 1 {
		integer, leftConversion, err := s.rewriter.manager.SequenceIntToStringArgument(left[0])
		if err != nil {
			return false, err
		}
		if leftConversion {
			if len(right) == 1 {
				other, rightConversion, err := s.rewriter.manager.SequenceIntToStringArgument(right[0])
				if err != nil {
					return false, err
				}
				if rightConversion {
					s.add(integer, other)
					return true, nil
				}
			}
			text, concrete, err := s.concreteString(right)
			if err != nil || !concrete {
				return false, err
			}
			value, valid := new(big.Int).SetString(text, 10)
			if !valid || value.String() != text {
				return false, nil
			}
			numeral, err := s.rewriter.makeInteger(value)
			if err != nil {
				return false, err
			}
			if err = s.hold(numeral); err != nil {
				return false, err
			}
			s.add(integer, numeral)
			return true, nil
		}
	}
	if len(right) == 1 && len(left) >= 1 {
		_, rightConversion, err := s.rewriter.manager.SequenceIntToStringArgument(right[0])
		if err != nil {
			return false, err
		}
		_, leftConversion, err := s.rewriter.manager.SequenceIntToStringArgument(left[0])
		if err != nil {
			return false, err
		}
		if rightConversion && !leftConversion {
			return s.solveIntToString(right, left)
		}
	}
	return false, nil
}

func (s *sequenceEqualityReduction) concatNonEmpty(parts []ast.ASTHandle, like ast.ASTHandle) (ast.ASTHandle, error) {
	kept := make([]ast.ASTHandle, 0, len(parts))
	for _, part := range parts {
		unit, err := s.rewriter.manager.IsSequenceUnit(part)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		_, stringValue, err := s.rewriter.manager.SequenceStringValue(part)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if unit || stringValue {
			kept = append(kept, part)
		}
	}
	return s.concat(kept, like)
}

func (s *sequenceEqualityReduction) lengthConstrained(left, right []ast.ASTHandle, leftLike, rightLike ast.ASTHandle) (handled, satisfiable bool, err error) {
	leftLength, leftBounded, err := s.rewriter.sequenceMinimumLength(left)
	if err != nil {
		return false, false, err
	}
	rightLength, rightBounded, err := s.rewriter.sequenceMinimumLength(right)
	if err != nil {
		return false, false, err
	}
	if leftBounded && leftLength < rightLength || rightBounded && rightLength < leftLength {
		return true, false, nil
	}
	bounded := leftBounded && leftLength == rightLength && leftLength > 0
	boundedRight := false
	if !bounded {
		boundedRight = rightBounded && leftLength == rightLength && leftLength > 0
		bounded = boundedRight
	}
	if !bounded {
		return false, true, nil
	}
	unknown := right
	if boundedRight {
		unknown = left
	}
	if _, err = s.setEmpty(unknown, false); err != nil {
		return false, false, err
	}
	leftValue, err := s.concatNonEmpty(left, leftLike)
	if err != nil {
		return false, false, err
	}
	rightValue, err := s.concatNonEmpty(right, rightLike)
	if err != nil {
		return false, false, err
	}
	s.add(leftValue, rightValue)
	return true, true, nil
}

func (s *sequenceEqualityReduction) alignSubsequence(left, right []ast.ASTHandle, leftLike, rightLike ast.ASTHandle) (handled, satisfiable bool, err error) {
	if len(left) == len(right) {
		return false, true, nil
	}
	if len(right) < len(left) {
		return s.alignSubsequence(right, left, rightLike, leftLike)
	}
	used := make([]bool, len(right))
	for _, part := range left {
		unit, err := s.rewriter.manager.IsSequenceUnit(part)
		if err != nil {
			return false, false, err
		}
		found := -1
		for j, candidate := range right {
			if used[j] {
				continue
			}
			candidateUnit := false
			if unit {
				candidateUnit, err = s.rewriter.manager.IsSequenceUnit(candidate)
				if err != nil {
					return false, false, err
				}
			}
			if part == candidate || unit && candidateUnit {
				found = j
				break
			}
		}
		if found < 0 {
			return false, true, nil
		}
		used[found] = true
	}
	changed := false
	matched := make([]ast.ASTHandle, 0, len(left))
	for j, part := range right {
		if used[j] {
			matched = append(matched, part)
			continue
		}
		satisfiable, err := s.setEmpty([]ast.ASTHandle{part}, true)
		if err != nil {
			return false, false, err
		}
		if !satisfiable {
			return true, false, nil
		}
		changed = true
	}
	if !changed {
		return false, true, nil
	}
	if len(left) > 0 {
		leftValue, err := s.concat(left, leftLike)
		if err != nil {
			return false, false, err
		}
		rightValue, err := s.concat(matched, rightLike)
		if err != nil {
			return false, false, err
		}
		s.add(leftValue, rightValue)
	}
	return true, true, nil
}

func (s *sequenceEqualityReduction) setEmpty(parts []ast.ASTHandle, all bool) (bool, error) {
	var empty ast.ASTHandle
	for _, part := range parts {
		unit, err := s.rewriter.manager.IsSequenceUnit(part)
		if err != nil {
			return false, err
		}
		if unit {
			if all {
				return false, nil
			}
			continue
		}
		isEmpty, err := s.rewriter.manager.IsSequenceEmpty(part)
		if err != nil {
			return false, err
		}
		if isEmpty {
			continue
		}
		text, stringValue, err := s.rewriter.manager.SequenceStringValue(part)
		if err != nil {
			return false, err
		}
		if stringValue {
			if all && text != "" {
				return false, nil
			}
			continue
		}
		if empty == (ast.ASTHandle{}) {
			sort, err := s.rewriter.manager.GetSort(part)
			if err != nil {
				return false, err
			}
			empty, err = s.rewriter.manager.MkSequenceEmpty(sort)
			if err != nil {
				return false, err
			}
			if err = s.hold(empty); err != nil {
				return false, err
			}
		}
		s.add(empty, part)
	}
	return true, nil
}

func (s *sequenceEqualityReduction) publish(out ast.ASTRef) (status RewriteStatus, err error) {
	if len(s.left) != len(s.right) {
		return RewriteFailed, ast.ErrASTDomain
	}
	if s.parts != nil {
		s.parts.Changed = true
		s.parts.Left, s.parts.Right = s.residualLeft, s.residualRight
		s.parts.EqualitiesLeft, s.parts.EqualitiesRight = s.left, s.right
		s.parts.refs = append(s.parts.refs, s.refs...)
		s.refs = nil
		return Rewrite3, nil
	}
	equalities := make([]ast.ASTHandle, 0, len(s.left))
	for i := range s.left {
		equality, err := s.rewriter.manager.MkEq(s.left[i], s.right[i])
		if err != nil {
			return RewriteFailed, err
		}
		if err = s.hold(equality); err != nil {
			return RewriteFailed, err
		}
		equalities = append(equalities, equality)
	}
	result, err := s.rewriter.manager.MkAndArray(equalities)
	if err != nil {
		return RewriteFailed, err
	}
	if err = s.rewriter.manager.SetRef(out, result); err != nil {
		return RewriteFailed, err
	}
	return Rewrite3, nil
}

func (s *sequenceEqualityReduction) inconsistent(out ast.ASTRef) (RewriteStatus, error) {
	if s.parts != nil {
		s.parts.Inconsistent, s.parts.Changed = true, true
		return RewriteDone, nil
	}
	return s.rewriter.assignBoolean(false, out)
}

// RewriteEquality ports the boundary cancellation, concrete String splitting,
// unit alignment, and empty-residual portion of seq_rewriter::mk_eq_core.
func (r *SequenceRewriter) RewriteEquality(left, right ast.ASTHandle, out ast.ASTRef) (status RewriteStatus, err error) {
	return r.rewriteEquality(left, right, out, nil)
}

func (r *SequenceRewriter) rewriteEquality(left, right ast.ASTHandle, out ast.ASTRef, parts *SequenceEqualityReduction) (status RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return RewriteFailed, err
	}
	ls, err := r.manager.SequenceFlattenConcat(left)
	if err != nil {
		return RewriteFailed, err
	}
	rs, err := r.manager.SequenceFlattenConcat(right)
	if err != nil {
		return RewriteFailed, err
	}
	state := sequenceEqualityReduction{rewriter: r, parts: parts}
	defer func() { err = errors.Join(err, state.close()) }()
	if parts != nil {
		parts.Left, parts.Right = append([]ast.ASTHandle(nil), ls...), append([]ast.ASTHandle(nil), rs...)
		for _, input := range []ast.ASTHandle{left, right} {
			ref, err := r.manager.NewRef(input)
			if err != nil {
				return RewriteFailed, err
			}
			parts.refs = append(parts.refs, ref)
		}
	}
	changed := false
	localChange := false

	for len(ls) > 0 && len(rs) > 0 {
		l, rr := ls[len(ls)-1], rs[len(rs)-1]
		_, rightUnit, e := r.manager.SequenceUnitArgument(rr)
		if e != nil {
			return RewriteFailed, e
		}
		_, leftString, e := r.manager.SequenceStringValue(l)
		if e != nil {
			return RewriteFailed, e
		}
		if rightUnit && leftString {
			ls, rs = rs, ls
			l, rr = ls[len(ls)-1], rs[len(rs)-1]
		}
		if l == rr {
			ls, rs = ls[:len(ls)-1], rs[:len(rs)-1]
			changed, localChange = true, true
			continue
		}
		leftElement, leftUnit, e := r.manager.SequenceUnitArgument(l)
		if e != nil {
			return RewriteFailed, e
		}
		rightElement, rightUnit, e := r.manager.SequenceUnitArgument(rr)
		if e != nil {
			return RewriteFailed, e
		}
		if leftUnit && rightUnit {
			distinct, e := r.manager.AreDistinct(leftElement, rightElement)
			if e != nil {
				return RewriteFailed, e
			}
			if distinct {
				return state.inconsistent(out)
			}
			state.add(leftElement, rightElement)
			ls, rs = ls[:len(ls)-1], rs[:len(rs)-1]
			changed, localChange = true, true
			continue
		}
		text, stringValue, e := r.manager.SequenceStringValue(rr)
		if e != nil {
			return RewriteFailed, e
		}
		if !leftUnit || !stringValue || text == "" {
			break
		}
		character, e := state.character(text[len(text)-1])
		if e != nil {
			return RewriteFailed, e
		}
		state.add(character, leftElement)
		ls = ls[:len(ls)-1]
		if len(text) == 1 {
			rs = rs[:len(rs)-1]
		} else {
			rs[len(rs)-1], e = state.stringValue(text[:len(text)-1])
			if e != nil {
				return RewriteFailed, e
			}
		}
		changed, localChange = true, true
	}

	headLeft, headRight := 0, 0
	for headLeft < len(ls) && headRight < len(rs) {
		l, rr := ls[headLeft], rs[headRight]
		_, rightUnit, e := r.manager.SequenceUnitArgument(rr)
		if e != nil {
			return RewriteFailed, e
		}
		_, leftString, e := r.manager.SequenceStringValue(l)
		if e != nil {
			return RewriteFailed, e
		}
		if rightUnit && leftString {
			ls, rs = rs, ls
			headLeft, headRight = headRight, headLeft
			l, rr = ls[headLeft], rs[headRight]
		}
		if l == rr {
			headLeft++
			headRight++
			changed, localChange = true, true
			continue
		}
		leftElement, leftUnit, e := r.manager.SequenceUnitArgument(l)
		if e != nil {
			return RewriteFailed, e
		}
		rightElement, rightUnit, e := r.manager.SequenceUnitArgument(rr)
		if e != nil {
			return RewriteFailed, e
		}
		if leftUnit && rightUnit {
			distinct, e := r.manager.AreDistinct(leftElement, rightElement)
			if e != nil {
				return RewriteFailed, e
			}
			if distinct {
				return state.inconsistent(out)
			}
			state.add(leftElement, rightElement)
			headLeft++
			headRight++
			changed, localChange = true, true
			continue
		}
		text, stringValue, e := r.manager.SequenceStringValue(rr)
		if e != nil {
			return RewriteFailed, e
		}
		if !leftUnit || !stringValue || text == "" {
			break
		}
		character, e := state.character(text[0])
		if e != nil {
			return RewriteFailed, e
		}
		state.add(character, leftElement)
		headLeft++
		if len(text) == 1 {
			headRight++
		} else {
			rs[headRight], e = state.stringValue(text[1:])
			if e != nil {
				return RewriteFailed, e
			}
		}
		changed, localChange = true, true
	}

	for headLeft < len(ls) && headRight < len(rs) {
		leftText, leftString, e := r.manager.SequenceStringValue(ls[headLeft])
		if e != nil {
			return RewriteFailed, e
		}
		rightText, rightString, e := r.manager.SequenceStringValue(rs[headRight])
		if e != nil {
			return RewriteFailed, e
		}
		if !leftString || !rightString {
			break
		}
		common := len(leftText)
		if len(rightText) < common {
			common = len(rightText)
		}
		if leftText[:common] != rightText[:common] {
			return state.inconsistent(out)
		}
		if common == len(leftText) {
			headLeft++
		} else {
			ls[headLeft], e = state.stringValue(leftText[common:])
			if e != nil {
				return RewriteFailed, e
			}
		}
		if common == len(rightText) {
			headRight++
		} else {
			rs[headRight], e = state.stringValue(rightText[common:])
			if e != nil {
				return RewriteFailed, e
			}
		}
		changed, localChange = true, true
	}

	for headLeft < len(ls) && headRight < len(rs) {
		leftText, leftString, e := r.manager.SequenceStringValue(ls[len(ls)-1])
		if e != nil {
			return RewriteFailed, e
		}
		rightText, rightString, e := r.manager.SequenceStringValue(rs[len(rs)-1])
		if e != nil {
			return RewriteFailed, e
		}
		if !leftString || !rightString {
			break
		}
		common := len(leftText)
		if len(rightText) < common {
			common = len(rightText)
		}
		if leftText[len(leftText)-common:] != rightText[len(rightText)-common:] {
			return state.inconsistent(out)
		}
		ls, rs = ls[:len(ls)-1], rs[:len(rs)-1]
		if common < len(leftText) {
			remaining, e := state.stringValue(leftText[:len(leftText)-common])
			if e != nil {
				return RewriteFailed, e
			}
			ls = append(ls, remaining)
		}
		if common < len(rightText) {
			remaining, e := state.stringValue(rightText[:len(rightText)-common])
			if e != nil {
				return RewriteFailed, e
			}
			rs = append(rs, remaining)
		}
		changed, localChange = true, true
	}

	remainingLeft, remainingRight := ls[headLeft:], rs[headRight:]
	handled, e := state.solveIntToString(remainingLeft, remainingRight)
	if e != nil {
		return RewriteFailed, e
	}
	if handled {
		return state.publish(out)
	}
	handled, satisfiable, e := state.lengthConstrained(remainingLeft, remainingRight, left, right)
	if e != nil {
		return RewriteFailed, e
	}
	if handled {
		if !satisfiable {
			return state.inconsistent(out)
		}
		return state.publish(out)
	}
	if len(remainingLeft) == 0 && len(remainingRight) == 0 {
		if !changed {
			return RewriteFailed, nil
		}
		return state.publish(out)
	}
	if len(remainingLeft) == 0 || len(remainingRight) == 0 {
		parts := remainingRight
		if len(parts) == 0 {
			parts = remainingLeft
		}
		before := len(state.left)
		satisfiable, e := state.setEmpty(parts, true)
		if e != nil {
			return RewriteFailed, e
		}
		if !satisfiable {
			return state.inconsistent(out)
		}
		if len(parts) == 1 && !localChange {
			state.left = state.left[:before]
			state.right = state.right[:before]
			return RewriteFailed, nil
		}
		changed = changed || len(parts) > 1
		if !changed {
			return RewriteFailed, nil
		}
		return state.publish(out)
	}
	handled, satisfiable, e = state.alignSubsequence(remainingLeft, remainingRight, left, right)
	if e != nil {
		return RewriteFailed, e
	}
	if handled {
		if !satisfiable {
			return state.inconsistent(out)
		}
		return state.publish(out)
	}
	if !localChange {
		return RewriteFailed, nil
	}
	if parts != nil {
		state.residualLeft, state.residualRight = remainingLeft, remainingRight
		return state.publish(out)
	}
	leftResidual, e := state.concat(remainingLeft, left)
	if e != nil {
		return RewriteFailed, e
	}
	rightResidual, e := state.concat(remainingRight, right)
	if e != nil {
		return RewriteFailed, e
	}
	state.add(leftResidual, rightResidual)
	return state.publish(out)
}
