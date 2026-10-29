package smtcore

// Source: src/smt/theory_str.cpp get_const_str_asts_in_node and
// check_contain_by_eqc_val; the constant-value part of check_contain_in_new_eq.

import (
	"errors"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) stringConstantsInNode(expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	m := t.state.manager
	work := []ast.ASTHandle{expression}
	var constants []ast.ASTHandle
	for len(work) != 0 {
		last := len(work) - 1
		current := work[last]
		work = work[:last]
		if _, literal, err := m.SequenceStringValue(current); err != nil {
			return nil, err
		} else if literal {
			constants = append(constants, current)
			continue
		}
		kind, err := m.GetKind(current)
		if err != nil {
			return nil, err
		}
		if kind != ast.ASTApp {
			continue
		}
		count, err := m.GetNumArgs(current)
		if err != nil {
			return nil, err
		}
		for i := int(count) - 1; i >= 0; i-- {
			child, err := m.GetArg(current, uint32(i))
			if err != nil {
				return nil, err
			}
			work = append(work, child)
		}
	}
	return constants, nil
}

func (t *SMTStringTheory) checkContainByClassValue(variable, constant ast.ASTHandle) (result error) {
	m := t.state.manager
	text, literal, err := m.SequenceStringValue(constant)
	if err != nil || !literal {
		return err
	}
	for _, pair := range t.containmentPairs(variable) {
		haystack, needle := pair[0], pair[1]
		if variable != haystack && variable != needle {
			continue
		}
		premises := smtStringLengthPremises{theory: t}
		err := func() (result error) {
			defer func() { result = errors.Join(result, premises.close()) }()
			equality := func(left, right ast.ASTHandle) error {
				expression, err := m.MkEq(left, right)
				if err != nil {
					return err
				}
				return premises.add(expression)
			}
			if variable != constant {
				if err := equality(variable, constant); err != nil {
					return err
				}
			}
			other := haystack
			if variable == haystack {
				other = needle
			}
			value, known, err := t.eqcStringValue(other)
			if err != nil {
				return err
			}
			if value != other {
				if err := equality(other, value); err != nil {
					return err
				}
			}
			positive, decided := false, known
			if known {
				otherText, _, err := m.SequenceStringValue(value)
				if err != nil {
					return err
				}
				if variable == haystack {
					positive = strings.Contains(text, otherText)
				} else {
					positive = strings.Contains(otherText, text)
				}
			} else if variable == haystack {
				// A mandatory literal in any concatenation in the needle's
				// class can refute containment before the pending merge.
				for _, member := range t.stringClass(needle) {
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
					for _, piece := range pieces {
						part, _, err := m.SequenceStringValue(piece)
						if err != nil {
							return err
						}
						if !strings.Contains(text, part) {
							if member != needle {
								if err := equality(needle, member); err != nil {
									return err
								}
							}
							decided = true
							break
						}
					}
					if decided {
						break
					}
				}
			}
			if !decided {
				return nil
			}
			conclusion := t.containsByPair[pair]
			if !positive {
				conclusion, err = m.MkNot(conclusion)
				if err != nil {
					return err
				}
			}
			if err := premises.add(conclusion); err != nil {
				return err
			}
			guard, err := m.MkAndArray(premises.values[:len(premises.values)-1])
			if err != nil {
				return err
			}
			if err := premises.add(guard); err != nil {
				return err
			}
			negativeGuard, err := m.MkNot(guard)
			if err != nil {
				return err
			}
			if err := premises.add(negativeGuard); err != nil {
				return err
			}
			implication, err := m.MkOr(negativeGuard, conclusion)
			if err != nil {
				return err
			}
			return t.assertAxiom(implication)
		}()
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *SMTStringTheory) checkContainNewEquality(left, right ast.ASTHandle) error {
	if len(t.contains) == 0 {
		return nil
	}
	m := t.state.manager
	var constant ast.ASTHandle
	var members []ast.ASTHandle
	for _, side := range []ast.ASTHandle{left, right} {
		var lastConstant ast.ASTHandle
		for _, member := range t.stringClass(side) {
			members = append(members, member)
			if _, literal, err := m.SequenceStringValue(member); err != nil {
				return err
			} else if literal {
				lastConstant = member
			}
		}
		if constant == (ast.ASTHandle{}) {
			constant = lastConstant
		}
	}
	for _, member := range members {
		if constant.IsNull() {
			if err := t.checkContainBySubstring(member, members); err != nil {
				return err
			}
		} else if member != constant {
			if err := t.checkContainByClassValue(member, constant); err != nil {
				return err
			}
		}
	}
	for _, first := range members {
		for _, second := range members {
			if err := t.checkContainByEqualNodes(first, second); err != nil {
				return err
			}
		}
	}
	return nil
}
