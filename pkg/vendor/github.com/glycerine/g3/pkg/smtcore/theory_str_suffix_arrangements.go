package smtcore

// Source: src/smt/theory_str.cpp process_concat_eq_type2:
// concat(x,y) = concat(m,literal), in either operand orientation.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) stringBoundaryWitness(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	key := [2]ast.ASTHandle{left, right}
	for _, candidate := range [][2]ast.ASTHandle{key, {right, left}} {
		entry, found := t.arrangementVars[candidate]
		_, inScope := t.internalVariables[entry[0]]
		if !found || !inScope {
			continue
		}
		if err := t.refreshStringVariable(entry[0]); err != nil {
			return ast.ASTHandle{}, err
		}
		return entry[0], t.addNonemptyStringConstraint(entry[0])
	}
	variable, err := t.freshNonemptyString()
	if err != nil {
		return variable, err
	}
	flag, err := t.freshIntegerVariable("$$_xor")
	if err != nil {
		return variable, err
	}
	// Unlike type 1, native types 2/3/6 do not initialize a self-cut seed for
	// this witness. Its first merge creates a frame containing just origins.
	t.arrangementVars[key] = [3]ast.ASTHandle{variable, flag, {}}
	return variable, nil
}

func (t *SMTStringTheory) arrangeLiteralSuffixConcats(left, right ast.ASTHandle) (result error) {
	m := t.state.manager
	x, y, lc, err := m.SequenceConcatParts(left)
	if err != nil || !lc {
		return err
	}
	a, textTerm, rc, err := m.SequenceConcatParts(right)
	if err != nil || !rc {
		return err
	}
	if _, literal, err := m.SequenceStringValue(y); err != nil {
		return err
	} else if literal {
		x, y, a, textTerm = a, textTerm, x, y
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
	known := [4]bool{false, false, false, true}
	for i, expression := range []ast.ASTHandle{x, y, a} {
		values[i], known[i], err = t.lengthValue(expression)
		if err != nil {
			return err
		}
	}
	values[3] = new(big.Int).SetUint64(uint64(len(text)))
	witness, err := t.stringBoundaryWitness(left, right)
	if err != nil {
		return err
	}
	split := -1
	if known[0] && known[2] {
		split = values[0].Cmp(values[2]) + 1
	} else if known[1] {
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
	if split == 1 {
		var lengths []ast.ASTHandle
		for _, pair := range [][2]ast.ASTHandle{{x, a}, {y, textTerm}} {
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
		first, err := pin(m.MkEq(x, a))
		if err != nil {
			return err
		}
		second, err := pin(m.MkEq(y, textTerm))
		if err != nil {
			return err
		}
		conclusion, err := pin(m.MkAnd(first, second))
		if err != nil {
			return err
		}
		return t.publishStringArrangement(premise, conclusion, false)
	}
	if split == 0 {
		tail, err := pin(t.concatTerm(witness, textTerm))
		if err != nil {
			return err
		}
		possible, err := t.canTwoNodesEqual(y, tail)
		if err != nil || !possible {
			return err
		}
		guards := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, guards.close()) }()
		items := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, items.close()) }()
		if err := guards.add(parentEq); err != nil {
			return err
		}
		prefix, err := pin(t.concatTerm(x, witness))
		if err != nil {
			return err
		}
		if err := items.equate(a, prefix); err != nil {
			return err
		}
		if err := items.equate(y, tail); err != nil {
			return err
		}
		if known[0] && known[2] {
			if err := guards.length(x, values[0]); err != nil {
				return err
			}
			if err := guards.length(a, values[2]); err != nil {
				return err
			}
			if err := items.length(witness, new(big.Int).Sub(values[2], values[0])); err != nil {
				return err
			}
		} else {
			if err := guards.length(y, values[1]); err != nil {
				return err
			}
			if err := guards.length(textTerm, values[3]); err != nil {
				return err
			}
			if err := items.length(witness, new(big.Int).Sub(values[1], values[3])); err != nil {
				return err
			}
		}
		premise, err := pin(m.MkAndArray(guards.values))
		if err != nil {
			return err
		}
		conclusion, err := pin(m.MkAndArray(items.values))
		if err != nil {
			return err
		}
		if t.hasSelfCut(a, y) {
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
		for _, origin := range []ast.ASTHandle{y, a} {
			if err := t.mergeStringCut(witness, int64(t.level), origin); err != nil {
				return err
			}
		}
		return t.publishStringArrangement(premise, conclusion, parameters.StrongArrangements)
	}
	if split == 2 {
		guards := smtStringLengthPremises{theory: t}
		defer func() { result = errors.Join(result, guards.close()) }()
		if err := guards.add(parentEq); err != nil {
			return err
		}
		var difference *big.Int
		if known[0] && known[2] {
			if err := guards.length(x, values[0]); err != nil {
				return err
			}
			if err := guards.length(a, values[2]); err != nil {
				return err
			}
			difference = new(big.Int).Sub(values[0], values[2])
		} else {
			if err := guards.length(y, values[1]); err != nil {
				return err
			}
			difference = new(big.Int).Sub(values[3], values[1])
		}
		// The retained new_eq_check predecessor rejects lengths that cannot
		// fit the literal. Keep the native extraction precondition explicit.
		if difference.Sign() < 0 || !difference.IsUint64() || difference.Uint64() > uint64(len(text)) {
			return ast.ErrASTDomain
		}
		cut := int(difference.Uint64())
		prefixText, err := pin(t.stringLiteral(text[:cut]))
		if err != nil {
			return err
		}
		prefix, err := pin(t.concatTerm(a, prefixText))
		if err != nil {
			return err
		}
		remaining, err := pin(t.stringLiteral(text[cut:]))
		if err != nil {
			return err
		}
		prefixPossible, err := t.canTwoNodesEqual(x, prefix)
		if err != nil {
			return err
		}
		tailPossible := false
		if prefixPossible {
			tailPossible, err = t.canTwoNodesEqual(y, remaining)
			if err != nil {
				return err
			}
		}
		premise, err := pin(m.MkAndArray(guards.values))
		if err != nil {
			return err
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
		if err := items.equate(y, remaining); err != nil {
			return err
		}
		conclusion, err := m.MkAndArray(items.values)
		if err != nil {
			return err
		}
		return t.publishStringArrangement(premise, conclusion, parameters.StrongArrangements)
	}
	var options []ast.ASTHandle
	tail, err := pin(t.concatTerm(witness, textTerm))
	if err != nil {
		return err
	}
	possible, err := t.canTwoNodesEqual(y, tail)
	if err != nil {
		return err
	}
	if possible {
		var option ast.ASTHandle
		if t.hasSelfCut(a, y) {
			option, err = pin(t.stringOverlapAlternative(parameters))
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
			prefix, err := pin(t.concatTerm(x, witness))
			if err != nil {
				return err
			}
			items := smtStringLengthPremises{theory: t}
			defer func() { result = errors.Join(result, items.close()) }()
			if err := items.equate(a, prefix); err != nil {
				return err
			}
			if err := items.equate(y, tail); err != nil {
				return err
			}
			l, err := pin(m.MkSequenceLength(x))
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
			option, err = pin(m.MkAndArray(items.values))
			if err != nil {
				return err
			}
			options = append(options, option)
			if err := t.addStringBranchPriority(option, 0.1); err != nil {
				return err
			}
			for _, origin := range []ast.ASTHandle{y, a} {
				if err := t.mergeStringCut(witness, int64(t.state.scopeLevel), origin); err != nil {
					return err
				}
			}
		}
	}
	for cut := 0; cut <= len(text); cut++ {
		prefixText, err := pin(t.stringLiteral(text[:cut]))
		if err != nil {
			return err
		}
		prefix, err := pin(t.concatTerm(a, prefixText))
		if err != nil {
			return err
		}
		remaining, err := pin(t.stringLiteral(text[cut:]))
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
		tailPossible, err := t.canTwoNodesEqual(y, remaining)
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
		if err := items.equate(y, remaining); err != nil {
			return errors.Join(err, items.close())
		}
		if err := items.length(y, big.NewInt(int64(len(text)-cut))); err != nil {
			return errors.Join(err, items.close())
		}
		option, err := pin(m.MkAndArray(items.values))
		if err = errors.Join(err, items.close()); err != nil {
			return err
		}
		options = append(options, option)
		priority := 0.1
		if cut == 0 {
			priority = 0.5
		}
		if err := t.addStringBranchPriority(option, priority); err != nil {
			return err
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
