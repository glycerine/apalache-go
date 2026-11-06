package smtcore

// Source: src/smt/theory_str.cpp in_same_eqc, simplify_concat,
// group_terms_by_eqc and the initial/coherence/concat steps of handle_equality.

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

func stringSortHandles(values []ast.ASTHandle) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Slot() != values[j].Slot() {
			return values[i].Slot() < values[j].Slot()
		}
		return values[i].Generation() < values[j].Generation()
	})
}

func (t *SMTStringTheory) sameStringClass(left, right ast.ASTHandle) (bool, error) {
	if left == right {
		return true, nil
	}
	for _, expression := range []ast.ASTHandle{left, right} {
		if err := t.state.Internalize(expression, false); err != nil {
			return false, err
		}
	}
	for _, member := range t.stringClass(left) {
		if member == right {
			return true, nil
		}
	}
	return false, nil
}

func (t *SMTStringTheory) simplifyStringConcat(expression ast.ASTHandle) (out ast.ASTHandle, result error) {
	leaves, err := t.concatLeaves(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	resolved := make(map[ast.ASTHandle]ast.ASTHandle)
	for _, leaf := range leaves {
		value, _, err := t.eqcStringValue(leaf)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if value != leaf {
			resolved[leaf] = value
		}
	}
	if len(resolved) == 0 {
		return expression, nil
	}
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	out, err = t.stringLiteral("")
	if err != nil {
		return out, err
	}
	// Keep every intermediate concat alive until the result is internalized.
	if err := premises.add(out); err != nil {
		return out, err
	}
	for _, leaf := range leaves {
		value, _, err := t.eqcStringValue(leaf)
		if err != nil {
			return out, err
		}
		out, err = t.concatTerm(out, value)
		if err != nil {
			return out, err
		}
		if err := premises.add(out); err != nil {
			return out, err
		}
	}
	same, err := t.sameStringClass(expression, out)
	if err != nil || same {
		return out, err
	}
	guards := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, guards.close()) }()
	var keys []ast.ASTHandle
	for key := range resolved {
		keys = append(keys, key)
	}
	stringSortHandles(keys)
	for _, key := range keys {
		if err := guards.equate(key, resolved[key]); err != nil {
			return out, err
		}
	}
	equality, err := t.state.manager.MkEq(expression, out)
	if err != nil {
		return out, err
	}
	return out, guards.imply(equality)
}

type smtStringClassGroups struct {
	concats, variables, constants []ast.ASTHandle
}

func (t *SMTStringTheory) groupStringClass(expression ast.ASTHandle) (groups smtStringClassGroups, result error) {
	m := t.state.manager
	seen := make(map[ast.ASTHandle]struct{})
	for _, member := range t.stringClass(expression) {
		concat, err := m.IsSequenceConcat(member)
		if err != nil {
			return groups, err
		}
		if concat {
			member, err = t.simplifyStringConcat(member)
			if err != nil {
				return groups, err
			}
		}
		if _, found := seen[member]; found {
			continue
		}
		seen[member] = struct{}{}
		concat, err = m.IsSequenceConcat(member)
		if err != nil {
			return groups, err
		}
		_, constant, err := m.SequenceStringValue(member)
		if err != nil {
			return groups, err
		}
		switch {
		case concat:
			groups.concats = append(groups.concats, member)
		case constant:
			groups.constants = append(groups.constants, member)
		default:
			groups.variables = append(groups.variables, member)
		}
	}
	stringSortHandles(groups.concats)
	stringSortHandles(groups.variables)
	stringSortHandles(groups.constants)
	return groups, nil
}

func (t *SMTStringTheory) handleConstantEqualityStages(left, right ast.ASTHandle) error {
	if err := t.stringZeroLengthCoherence(left, right); err != nil {
		return err
	}
	if err := t.assertStringLengthEquality(left, right); err != nil {
		return err
	}
	l, err := t.groupStringClass(left)
	if err != nil {
		return err
	}
	r, err := t.groupStringClass(right)
	if err != nil {
		return err
	}
	if err := t.simplifyStringConcatGroups(l, r); err != nil {
		return err
	}
	// Source step 2 follows the quick paths and all six concat arrangements.
	if len(l.constants) != 0 {
		for _, concat := range r.concats {
			if err := t.solveConcatConstant(concat, l.constants[0]); err != nil {
				return err
			}
		}
	} else if len(r.constants) != 0 {
		for _, concat := range l.concats {
			if err := t.solveConcatConstant(concat, r.constants[0]); err != nil {
				return err
			}
		}
	}
	if err := t.simplifyEqualityParents(left, right); err != nil {
		return err
	}
	return t.processEqualityUnrollConstants(left, right)
}

func (t *SMTStringTheory) stringZeroLengthCoherence(left, right ast.ASTHandle) (result error) {
	m := t.state.manager
	lv, lk, err := t.lengthValue(left)
	if err != nil {
		return err
	}
	rv, rk, err := t.lengthValue(right)
	if err != nil {
		return err
	}
	empty, err := t.stringLiteral("")
	if err != nil {
		return err
	}
	refs := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, refs.close()) }()
	if err := refs.add(empty); err != nil {
		return err
	}
	for i, pair := range [][2]ast.ASTHandle{{left, right}, {right, left}} {
		knownZero := lk && lv.Sign() == 0
		if i == 1 {
			knownZero = rk && rv.Sign() == 0
		}
		if !knownZero || pair[1] == empty {
			continue
		}
		same, err := t.sameStringClass(pair[0], empty)
		if err != nil {
			return err
		}
		if same {
			continue
		}
		length, err := m.MkSequenceLength(pair[0])
		if err != nil {
			return err
		}
		if err := refs.add(length); err != nil {
			return err
		}
		zero, err := m.ArithmeticIntegerNumeralText("0")
		if err != nil {
			return err
		}
		eqLength, err := m.MkEq(length, zero)
		if err != nil {
			return err
		}
		if err := refs.add(eqLength); err != nil {
			return err
		}
		eqEmpty, err := m.MkEq(pair[0], empty)
		if err != nil {
			return err
		}
		if err := refs.add(eqEmpty); err != nil {
			return err
		}
		axiom, err := m.MkEq(eqLength, eqEmpty)
		if err != nil {
			return err
		}
		if err := t.assertAxiom(axiom); err != nil {
			return err
		}
	}
	return nil
}
