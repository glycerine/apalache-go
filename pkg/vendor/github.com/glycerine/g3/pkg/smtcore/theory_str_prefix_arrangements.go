package smtcore

// Source: src/smt/theory_str.cpp process_concat_eq_type3:
// concat(x,y) = concat(literal,n), in either operand orientation.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) arrangeLiteralPrefixConcats(left, right ast.ASTHandle) (result error) {
	m := t.state.manager
	x, y, lc, err := m.SequenceConcatParts(left)
	if err != nil || !lc {
		return err
	}
	textTerm, n, rc, err := m.SequenceConcatParts(right)
	if err != nil || !rc {
		return err
	}
	if _, literal, err := m.SequenceStringValue(x); err != nil {
		return err
	} else if literal {
		x, y, textTerm, n = textTerm, n, x, y
	}
	text, literal, err := m.SequenceStringValue(textTerm)
	if err != nil || !literal {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	parameters := SMTStrState{StrongArrangements: true, OverlapTheoryAwarePriority: -0.1}
	if t.parameterOwner != nil {
		parameters, err = t.stringParameterState()
		if err != nil {
			return err
		}
	}
	values := [4]*big.Int{}
	known := [4]bool{false, false, true, false}
	terms := [4]ast.ASTHandle{x, y, textTerm, n}
	for _, i := range []int{0, 1, 3} {
		values[i], known[i], err = t.lengthValue(terms[i])
		if err != nil {
			return err
		}
	}
	values[2] = new(big.Int).SetUint64(uint64(len(text)))
	witness, err := t.stringBoundaryWitness(left, right)
	if err != nil {
		return err
	}
	split := -1
	if known[0] {
		split = values[0].Cmp(values[2]) + 1
	} else if known[1] && known[3] {
		split = 1 - values[1].Cmp(values[3])
	}
	locals := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, locals.close()) }()
	pin := func(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
		if err != nil {
			return value, err
		}
		return value, locals.add(value)
	}
	parentEq, err := pin(m.MkEq(left, right))
	if err != nil {
		return err
	}
	if split == 0 {
		guards := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, guards.close()) }()
		if err := guards.add(parentEq); err != nil {
			return err
		}
		prefixLength := values[0]
		if !known[0] {
			prefixLength = new(big.Int).Sub(values[2], new(big.Int).Sub(values[1], values[3]))
			if err := guards.length(y, values[1]); err != nil {
				return err
			}
			if err := guards.length(n, values[3]); err != nil {
				return err
			}
		} else if err := guards.length(x, values[0]); err != nil {
			return err
		}
		// Native extraction requires a nonnegative prefix fitting this
		// literal. Retained equality consistency precedes this algorithm.
		if prefixLength.Sign() < 0 || !prefixLength.IsUint64() || prefixLength.Uint64() > uint64(len(text)) {
			return ast.ErrASTDomain
		}
		cut := int(prefixLength.Uint64())
		prefix, err := pin(t.stringLiteral(text[:cut]))
		if err != nil {
			return err
		}
		suffix, err := pin(t.stringLiteral(text[cut:]))
		if err != nil {
			return err
		}
		premise, err := pin(m.MkAndArray(guards.values))
		if err != nil {
			return err
		}
		tail, err := pin(t.concatTerm(suffix, n))
		if err != nil {
			return err
		}
		prefixPossible, err := t.canTwoNodesEqual(x, prefix)
		if err != nil {
			return err
		}
		tailPossible := false
		if prefixPossible {
			tailPossible, err = t.canTwoNodesEqual(y, tail)
			if err != nil {
				return err
			}
		}
		if !prefixPossible || !tailPossible {
			negative, err := m.MkNot(premise)
			if err != nil {
				return err
			}
			return t.assertAxiom(negative)
		}
		items := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, items.close()) }()
		if err := items.equate(x, prefix); err != nil {
			return err
		}
		if err := items.equate(y, tail); err != nil {
			return err
		}
		conclusion, err := m.MkAndArray(items.values)
		if err != nil {
			return err
		}
		return t.publishStringArrangement(premise, conclusion, parameters.StrongArrangements)
	}
	if split == 1 {
		var lengths []ast.ASTHandle
		for _, pair := range [][2]ast.ASTHandle{{x, textTerm}, {y, n}} {
			l, err := pin(m.MkSequenceLength(pair[0]))
			if err != nil {
				return err
			}
			r, err := pin(m.MkSequenceLength(pair[1]))
			if err != nil {
				return err
			}
			eq, err := pin(m.MkEq(l, r))
			if err != nil {
				return err
			}
			lengths = append(lengths, eq)
		}
		equalLength, err := pin(m.MkOrArray(lengths))
		if err != nil {
			return err
		}
		premise, err := pin(m.MkAnd(parentEq, equalLength))
		if err != nil {
			return err
		}
		first, err := pin(m.MkEq(x, textTerm))
		if err != nil {
			return err
		}
		second, err := pin(m.MkEq(y, n))
		if err != nil {
			return err
		}
		conclusion, err := pin(m.MkAnd(first, second))
		if err != nil {
			return err
		}
		// Unlike family 2's alignment implication, the source family 3
		// handler applies StrongArrangements to this branch as well.
		return t.publishStringArrangement(premise, conclusion, parameters.StrongArrangements)
	}
	if split == 2 {
		guards := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, guards.close()) }()
		if err := guards.add(parentEq); err != nil {
			return err
		}
		var residualLength *big.Int
		if !known[0] {
			residualLength = new(big.Int).Sub(values[3], values[1])
			if err := guards.length(y, values[1]); err != nil {
				return err
			}
			if err := guards.length(n, values[3]); err != nil {
				return err
			}
		} else {
			residualLength = new(big.Int).Sub(values[0], values[2])
			if err := guards.length(x, values[0]); err != nil {
				return err
			}
		}
		premise, err := pin(m.MkAndArray(guards.values))
		if err != nil {
			return err
		}
		prefix, err := pin(t.concatTerm(textTerm, witness))
		if err != nil {
			return err
		}
		tail, err := pin(t.concatTerm(witness, y))
		if err != nil {
			return err
		}
		possible, err := t.canTwoNodesEqual(x, prefix)
		if err != nil || !possible {
			return err // Native negation of this failed feasibility is disabled.
		}
		if t.hasSelfCut(x, n) {
			tester, err := t.stringOverlapAlternative(parameters)
			if err != nil {
				return err
			}
			if err := t.publishStringArrangement(premise, tester, false); err != nil {
				return err
			}
			if parameters.FiniteOverlapModels {
				return t.addStringBranchPriority(tester, parameters.OverlapTheoryAwarePriority)
			}
			return nil
		}
		items := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, items.close()) }()
		if err := items.equate(x, prefix); err != nil {
			return err
		}
		if err := items.equate(n, tail); err != nil {
			return err
		}
		if err := items.length(witness, residualLength); err != nil {
			return err
		}
		conclusion, err := pin(m.MkAndArray(items.values))
		if err != nil {
			return err
		}
		for _, origin := range []ast.ASTHandle{x, n} {
			if err := t.mergeStringCut(witness, int64(t.level), origin); err != nil {
				return err
			}
		}
		return t.publishStringArrangement(premise, conclusion, parameters.StrongArrangements)
	}
	var options []ast.ASTHandle
	for cut := 0; cut <= len(text); cut++ {
		prefix, err := pin(t.stringLiteral(text[:cut]))
		if err != nil {
			return err
		}
		suffix, err := pin(t.stringLiteral(text[cut:]))
		if err != nil {
			return err
		}
		tail, err := pin(t.concatTerm(suffix, n))
		if err != nil {
			return err
		}
		prefixPossible, err := t.canTwoNodesEqual(x, prefix)
		if err != nil {
			return err
		}
		if !prefixPossible {
			continue
		}
		tailPossible, err := t.canTwoNodesEqual(y, tail)
		if err != nil {
			return err
		}
		if !tailPossible {
			continue
		}
		items := smtStringLengthPremises{theory: t}
		if err := items.equate(x, prefix); err != nil {
			return errors.Join(err, items.close())
		}
		if err := items.equate(y, tail); err != nil {
			return errors.Join(err, items.close())
		}
		length, err := pin(m.MkSequenceLength(x))
		if err != nil {
			return errors.Join(err, items.close())
		}
		cropLength, err := pin(m.MkSequenceLength(prefix))
		if err != nil {
			return errors.Join(err, items.close())
		}
		if err := items.equate(length, cropLength); err != nil {
			return errors.Join(err, items.close())
		}
		option, err := pin(m.MkAndArray(items.values))
		if err = errors.Join(err, items.close()); err != nil {
			return err
		}
		options = append(options, option)
		priority := 0.1
		if cut == len(text) {
			priority = 0.5
		}
		if err := t.addStringBranchPriority(option, priority); err != nil {
			return err
		}
	}
	prefix, err := pin(t.concatTerm(textTerm, witness))
	if err != nil {
		return err
	}
	possible, err := t.canTwoNodesEqual(x, prefix)
	if err != nil {
		return err
	}
	if possible {
		if t.hasSelfCut(x, n) {
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
		} else {
			items := smtStringLengthPremises{theory: t}
			defer func() { result = errors.Join(result, items.close()) }()
			tail, err := pin(t.concatTerm(witness, y))
			if err != nil {
				return err
			}
			if err := items.equate(x, prefix); err != nil {
				return err
			}
			if err := items.equate(n, tail); err != nil {
				return err
			}
			l, err := pin(m.MkSequenceLength(textTerm))
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
			total, err := pin(m.MkSequenceLength(x))
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
			for _, origin := range []ast.ASTHandle{x, n} {
				if err := t.mergeStringCut(witness, int64(t.level), origin); err != nil {
					return err
				}
			}
		}
	}
	if len(options) == 0 {
		return nil
	}
	conclusion, err := pin(m.MkOrArray(options))
	if err != nil {
		return err
	}
	if err := t.publishStringArrangement(parentEq, conclusion, parameters.StrongArrangements); err != nil {
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
