package smtcore

// Source: src/smt/theory_str.cpp check_contain_by_substr,
// check_contain_by_eq_nodes and check_contain_in_new_eq.

import (
	"errors"
	"sort"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

// Native pair indexes are sets of expression pointers. Use stable handle
// order for their traversal rather than Go map iteration order.
func (t *SMTStringTheory) containmentPairs(expression ast.ASTHandle) [][2]ast.ASTHandle {
	var pairs [][2]ast.ASTHandle
	for pair := range t.containsIndex[expression] {
		pairs = append(pairs, pair)
	}
	sort.Slice(pairs, func(i, j int) bool {
		for k := 0; k < 2; k++ {
			if pairs[i][k].Slot() != pairs[j][k].Slot() {
				return pairs[i][k].Slot() < pairs[j][k].Slot()
			}
			if pairs[i][k].Generation() != pairs[j][k].Generation() {
				return pairs[i][k].Generation() < pairs[j][k].Generation()
			}
		}
		return false
	})
	return pairs
}

func (p *smtStringLengthPremises) equate(left, right ast.ASTHandle) error {
	if left == right {
		return nil
	}
	equality, err := p.theory.state.manager.MkEq(left, right)
	if err != nil {
		return err
	}
	return p.add(equality)
}

// Publish without changing the premise list: check_contain_by_substr keeps
// its vector across index entries, including earlier equalities.
func (p *smtStringLengthPremises) imply(conclusion ast.ASTHandle) (result error) {
	m := p.theory.state.manager
	locals := smtStringLengthPremises{theory: p.theory}
	defer func() { result = errors.Join(result, locals.close()) }()
	if err := locals.add(conclusion); err != nil {
		return err
	}
	if len(p.values) == 0 {
		return p.theory.assertAxiom(conclusion)
	}
	guard, err := m.MkAndArray(p.values)
	if err != nil {
		return err
	}
	if err := locals.add(guard); err != nil {
		return err
	}
	negative, err := m.MkNot(guard)
	if err != nil {
		return err
	}
	if err := locals.add(negative); err != nil {
		return err
	}
	axiom, err := m.MkOr(negative, conclusion)
	if err != nil {
		return err
	}
	return p.theory.assertAxiom(axiom)
}

func (t *SMTStringTheory) checkContainBySubstring(variable ast.ASTHandle, prospective []ast.ASTHandle) (result error) {
	m := t.state.manager
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	for _, pair := range t.containmentPairs(variable) {
		if pair[1] != variable {
			continue
		}
		value, known, err := t.eqcStringValue(pair[0])
		if err != nil {
			return err
		}
		if !known {
			continue
		}
		if err := premises.equate(pair[0], value); err != nil {
			return err
		}
		text, _, err := m.SequenceStringValue(value)
		if err != nil {
			return err
		}
		for _, member := range prospective {
			concat, err := m.IsSequenceConcat(member)
			if err != nil {
				return err
			}
			if !concat {
				continue
			}
			pieces, err := t.stringConstantsInNode(member)
			if err != nil {
				return err
			}
			refused := false
			for _, piece := range pieces {
				part, _, err := m.SequenceStringValue(piece)
				if err != nil {
					return err
				}
				if !strings.Contains(text, part) {
					if err := premises.equate(variable, member); err != nil {
						return err
					}
					negative, err := m.MkNot(t.containsByPair[pair])
					if err != nil {
						return err
					}
					if err := premises.imply(negative); err != nil {
						return err
					}
					refused = true
					break
				}
			}
			if refused {
				break
			}
		}
	}
	return nil
}

func (t *SMTStringTheory) checkContainByEqualNodes(left, right ast.ASTHandle) error {
	firstPairs, secondPairs := t.containmentPairs(left), t.containmentPairs(right)
	if len(firstPairs) == 0 || len(secondPairs) == 0 {
		return nil
	}
	for _, first := range firstPairs {
		if first == [2]ast.ASTHandle{left, right} {
			if err := t.containmentReflexive(left, right, t.containsByPair[first]); err != nil {
				return err
			}
		}
		for _, second := range secondPairs {
			if first == second {
				continue
			}
			if first[0] == left && second[0] == right {
				if err := t.checkContainPair(left, right, first, second, true); err != nil {
					return err
				}
			} else if first[1] == left && second[1] == right {
				if err := t.checkContainPair(left, right, first, second, false); err != nil {
					return err
				}
			}
		}
		// Preserve the native identical-node early exit after its first key.
		if left == right {
			break
		}
	}
	return nil
}

func (t *SMTStringTheory) containmentReflexive(left, right, relation ast.ASTHandle) (result error) {
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	if err := premises.equate(left, right); err != nil {
		return err
	}
	return premises.imply(relation)
}

// When haystacks coincide, containing the larger needle implies containing
// the smaller one. When needles coincide, the implication direction reverses.
func (t *SMTStringTheory) checkContainPair(left, right ast.ASTHandle, first, second [2]ast.ASTHandle, equalHaystacks bool) (result error) {
	m := t.state.manager
	index := 0
	if equalHaystacks {
		index = 1
	}
	a, b := first[index], second[index]
	av, ak, err := t.eqcStringValue(a)
	if err != nil {
		return err
	}
	bv, bk, err := t.eqcStringValue(b)
	if err != nil {
		return err
	}
	if ak && bk {
		at, _, err := m.SequenceStringValue(av)
		if err != nil {
			return err
		}
		bt, _, err := m.SequenceStringValue(bv)
		if err != nil {
			return err
		}
		if at != bt && !strings.Contains(at, bt) && !strings.Contains(bt, at) {
			return nil
		}
		return t.containmentPairRule(left, right, a, av, b, bv, ast.ASTHandle{}, t.containsByPair[first], t.containsByPair[second], at == bt, equalHaystacks == strings.Contains(at, bt))
	}
	ac, bc := t.stringClass(a), t.stringClass(b)
	for _, member := range ac {
		if member == b {
			return t.containmentPairRule(left, right, a, b, b, b, ast.ASTHandle{}, t.containsByPair[first], t.containsByPair[second], true, true)
		}
	}
	for _, am := range ac {
		for _, bm := range bc {
			if condition, found := t.containsByPair[[2]ast.ASTHandle{am, bm}]; found {
				if err := t.containmentPairRule(left, right, a, am, b, bm, condition, t.containsByPair[first], t.containsByPair[second], false, equalHaystacks); err != nil {
					return err
				}
			}
			if condition, found := t.containsByPair[[2]ast.ASTHandle{bm, am}]; found {
				if err := t.containmentPairRule(left, right, a, am, b, bm, condition, t.containsByPair[first], t.containsByPair[second], false, !equalHaystacks); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (t *SMTStringTheory) containmentPairRule(left, right, a, av, b, bv, condition, first, second ast.ASTHandle, equivalent, forward bool) (result error) {
	m := t.state.manager
	premises := smtStringLengthPremises{theory: t}
	defer func() { result = errors.Join(result, premises.close()) }()
	for _, pair := range [][2]ast.ASTHandle{{left, right}, {a, av}, {b, bv}} {
		if err := premises.equate(pair[0], pair[1]); err != nil {
			return err
		}
	}
	if !condition.IsNull() {
		if err := premises.add(condition); err != nil {
			return err
		}
	}
	var conclusion ast.ASTHandle
	var err error
	if equivalent {
		conclusion, err = m.MkEq(first, second)
	} else {
		if !forward {
			first, second = second, first
		}
		negative, e := m.MkNot(first)
		if e != nil {
			return e
		}
		ref, e := m.NewRef(negative)
		if e != nil {
			return e
		}
		defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
		conclusion, err = m.MkOr(negative, second)
	}
	if err != nil {
		return err
	}
	return premises.imply(conclusion)
}
