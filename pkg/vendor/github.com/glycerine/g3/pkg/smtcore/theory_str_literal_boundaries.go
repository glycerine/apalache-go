package smtcore

// Source: src/smt/theory_str.cpp process_concat_eq_type4, type5 and type6.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// Families 4/5 cancel two literal prefixes/suffixes. Their residual placement
// and equality orientation differ; neither family introduces a witness/cut.
func (t *SMTStringTheory) arrangeMatchingLiteralBoundaries(left, right ast.ASTHandle, suffix bool) (result error) {
	m := t.state.manager
	l0, l1, lc, err := m.SequenceConcatParts(left)
	if err != nil || !lc {
		return err
	}
	r0, r1, rc, err := m.SequenceConcatParts(right)
	if err != nil || !rc {
		return err
	}
	firstTerm, secondTerm, firstVar, secondVar := l0, r0, l1, r1
	if suffix {
		firstTerm, secondTerm, firstVar, secondVar = l1, r1, l0, r0
	}
	first, literal, err := m.SequenceStringValue(firstTerm)
	if err != nil || !literal {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	second, literal, err := m.SequenceStringValue(secondTerm)
	if err != nil || !literal {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	common := len(first)
	if len(second) < common {
		common = len(second)
	}
	compatible := first[:common] == second[:common]
	if suffix {
		compatible = first[len(first)-common:] == second[len(second)-common:]
	}
	if !compatible {
		equality, err := pin(m.MkEq(left, right))
		if err != nil {
			return err
		}
		negative, err := m.MkNot(equality)
		if err != nil {
			return err
		}
		return t.assertAxiom(negative)
	}
	destination, replacement := secondVar, firstVar
	if len(first) == len(second) {
		if suffix {
			destination, replacement = firstVar, secondVar
		}
	} else {
		variable := firstVar
		var delta string
		if len(first) > len(second) {
			delta = first[len(second):]
		} else {
			destination, variable = firstVar, secondVar
			delta = second[len(first):]
		}
		if suffix {
			if len(first) > len(second) {
				delta = first[:len(first)-len(second)]
			} else {
				delta = second[:len(second)-len(first)]
			}
		}
		literal, err := pin(t.stringLiteral(delta))
		if err != nil {
			return err
		}
		if suffix {
			replacement, err = pin(t.concatTerm(variable, literal))
		} else {
			replacement, err = pin(t.concatTerm(literal, variable))
		}
		if err != nil {
			return err
		}
	}
	var same bool
	if !suffix && len(first) > len(second) {
		same, err = t.sameStringClass(replacement, destination)
	} else {
		same, err = t.sameStringClass(destination, replacement)
	}
	if err != nil || same {
		return err
	}
	conclusion, err := pin(m.MkEq(destination, replacement))
	if err != nil {
		return err
	}
	strong := true
	if t.parameterOwner != nil {
		parameters, err := t.stringParameterState()
		if err != nil {
			return err
		}
		strong = parameters.StrongArrangements
	}
	premise, err := pin(m.MkEq(left, right))
	if err != nil {
		return err
	}
	return t.publishStringArrangement(premise, conclusion, strong)
}

func (t *SMTStringTheory) arrangeOpposingLiteralBoundaries(left, right ast.ASTHandle) (result error) {
	m := t.state.manager
	firstTerm, y, lc, err := m.SequenceConcatParts(left)
	if err != nil || !lc {
		return err
	}
	a, secondTerm, rc, err := m.SequenceConcatParts(right)
	if err != nil || !rc {
		return err
	}
	if _, literal, err := m.SequenceStringValue(firstTerm); err != nil {
		return err
	} else if !literal {
		firstTerm, y, a, secondTerm = a, secondTerm, firstTerm, y
	}
	first, firstLiteral, err := m.SequenceStringValue(firstTerm)
	if err != nil {
		return err
	}
	second, secondLiteral, err := m.SequenceStringValue(secondTerm)
	if err != nil || !firstLiteral || !secondLiteral {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	// Source always includes zero overlap, then matching suffix/prefix
	// lengths in ascending order. It performs no further feasibility filter.
	overlaps := []int{0}
	for size := 1; size <= len(first) && size <= len(second); size++ {
		if first[len(first)-size:] == second[:size] {
			overlaps = append(overlaps, size)
		}
	}
	witness, err := t.stringBoundaryWitness(left, right)
	if err != nil {
		return err
	}
	parameters := SMTStrState{StrongArrangements: true, OverlapTheoryAwarePriority: -0.1}
	if t.parameterOwner != nil {
		parameters, err = t.stringParameterState()
		if err != nil {
			return err
		}
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	var options []ast.ASTHandle
	if !t.hasSelfCut(a, y) {
		items := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, items.close()) }()
		prefix, err := pin(t.concatTerm(firstTerm, witness))
		if err != nil {
			return err
		}
		if err := items.equate(a, prefix); err != nil {
			return err
		}
		tail, err := pin(t.concatTerm(witness, secondTerm))
		if err != nil {
			return err
		}
		if err := items.equate(y, tail); err != nil {
			return err
		}
		l, err := pin(m.MkSequenceLength(firstTerm))
		if err != nil {
			return err
		}
		r, err := pin(m.MkSequenceLength(witness))
		if err != nil {
			return err
		}
		sum, err := pin(m.MkArithmeticAdd(l, r))
		if err != nil {
			return err
		}
		total, err := pin(m.MkSequenceLength(a))
		if err != nil {
			return err
		}
		if err := items.equate(total, sum); err != nil {
			return err
		}
		option, err := pin(m.MkAndArray(items.values))
		if err != nil {
			return err
		}
		options = append(options, option)
		if err := t.addStringBranchPriority(option, 0.1); err != nil {
			return err
		}
		// Native family 6 intentionally does not merge cut origins into
		// commonVar, unlike the residual branches of families 1-3.
	} else {
		option, err := pin(t.stringOverlapAlternative(parameters))
		if err != nil {
			return err
		}
		options = append(options, option)
		if parameters.FiniteOverlapModels {
			if err := t.addStringBranchPriority(option, parameters.OverlapTheoryAwarePriority); err != nil {
				return err
			}
		}
	}
	for _, size := range overlaps {
		prefix, err := pin(t.stringLiteral(first[:len(first)-size]))
		if err != nil {
			return err
		}
		suffix, err := pin(t.stringLiteral(second[size:]))
		if err != nil {
			return err
		}
		items := smtStringLengthPremises{theory: t}
		if err := items.equate(a, prefix); err != nil {
			return errors.Join(err, items.close())
		}
		mLength, err := pin(m.MkSequenceLength(a))
		if err != nil {
			return errors.Join(err, items.close())
		}
		prefixLength, err := pin(m.MkSequenceLength(prefix))
		if err != nil {
			return errors.Join(err, items.close())
		}
		if err := items.equate(mLength, prefixLength); err != nil {
			return errors.Join(err, items.close())
		}
		if err := items.equate(y, suffix); err != nil {
			return errors.Join(err, items.close())
		}
		yLength, err := pin(m.MkSequenceLength(y))
		if err != nil {
			return errors.Join(err, items.close())
		}
		suffixLength, err := pin(m.MkSequenceLength(suffix))
		if err != nil {
			return errors.Join(err, items.close())
		}
		if err := items.equate(yLength, suffixLength); err != nil {
			return errors.Join(err, items.close())
		}
		option, err := pin(m.MkAndArray(items.values))
		if err = errors.Join(err, items.close()); err != nil {
			return err
		}
		options = append(options, option)
		priority := 0.1
		if size == 0 {
			priority = 0.5
		}
		if err := t.addStringBranchPriority(option, priority); err != nil {
			return err
		}
	}
	premise, err := pin(m.MkEq(left, right))
	if err != nil {
		return err
	}
	conclusion, err := pin(m.MkOrArray(options))
	if err != nil {
		return err
	}
	if err := t.publishStringArrangement(premise, conclusion, parameters.StrongArrangements); err != nil {
		return err
	}
	literals := make([]SMTLiteral, len(options))
	for i, option := range options {
		literals[i], err = t.state.Literal(option)
		if err != nil {
			return err
		}
	}
	return t.state.MakeTheoryCaseSplit(literals)
}
