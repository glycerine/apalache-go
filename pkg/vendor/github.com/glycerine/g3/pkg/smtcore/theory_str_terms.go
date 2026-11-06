package smtcore

// Source: src/smt/theory_str.cpp mk_fresh_const, mk_str_var,
// mk_concat_const_str, mk_concat and get_nodes_in_concat.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) freshStringVariable(name string) (ast.ASTHandle, error) {
	return t.createStringVariable(name, true)
}

func (t *SMTStringTheory) createStringVariable(name string, basicAxioms bool) (ast.ASTHandle, error) {
	m := t.state.manager
	// Native seq_util::str.mk_string_sort uses the internal _STRING_SORT.
	sort, err := m.MkFamilySort(t.FamilyID(), 2, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	freshName := name + "!tmp" + strconv.FormatUint(uint64(t.freshID), 10)
	t.freshID++
	symbol, err := m.Symbols.Intern(freshName)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	variable, err := m.MkSequenceSkolem(symbol, nil, sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.retain(variable); err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.state.Internalize(variable, false); err != nil {
		return ast.ASTHandle{}, err
	}
	node := t.state.context.enodes[variable]
	if _, err = t.mkVar(node); err != nil {
		return ast.ASTHandle{}, err
	}
	if basicAxioms {
		t.basicTodo = append(t.basicTodo, node)
	}
	t.variableSet[variable] = struct{}{}
	t.internalVariables[variable] = struct{}{}
	if t.variableScopes[t.level] == nil {
		t.variableScopes[t.level] = make(map[ast.ASTHandle]struct{})
	}
	t.variableScopes[t.level][variable] = struct{}{}
	return variable, nil
}

func (t *SMTStringTheory) concatConstantStrings(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	m := t.state.manager
	l, lc, err := t.eqcStringValue(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r, rc, err := t.eqcStringValue(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ls, literalLeft, err := m.SequenceStringValue(l)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rs, literalRight, err := m.SequenceStringValue(r)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	lc, rc = lc || literalLeft, rc || literalRight
	if lc && rc {
		return t.stringLiteral(ls + rs)
	}
	if lc && !rc && ls == "" {
		return right, nil
	}
	if !lc && rc && rs == "" {
		return left, nil
	}
	return ast.ASTHandle{}, nil
}

func (t *SMTStringTheory) concatTerm(left, right ast.ASTHandle) (ast.ASTHandle, error) {
	m := t.state.manager
	l, lc, err := t.eqcStringValue(left)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r, rc, err := t.eqcStringValue(right)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if lc && rc {
		return t.concatConstantStrings(l, r)
	}
	if lc && !rc {
		text, _, err := m.SequenceStringValue(l)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if text == "" {
			return r, nil
		}
		first, second, concat, err := m.SequenceConcatParts(r)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if concat {
			if _, literal, err := m.SequenceStringValue(first); err != nil {
				return ast.ASTHandle{}, err
			} else if literal {
				l, err = t.concatConstantStrings(l, first)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				r = second
			}
		}
	} else if !lc && rc {
		text, _, err := m.SequenceStringValue(r)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if text == "" {
			return l, nil
		}
		first, second, concat, err := m.SequenceConcatParts(l)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if concat {
			if _, literal, err := m.SequenceStringValue(second); err != nil {
				return ast.ASTHandle{}, err
			} else if literal {
				l = first
				r, err = t.concatConstantStrings(second, r)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
		}
	} else {
		l1, l2, leftConcat, err := m.SequenceConcatParts(l)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		r1, r2, rightConcat, err := m.SequenceConcatParts(r)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if leftConcat && rightConcat {
			_, leftLiteral, err := m.SequenceStringValue(l2)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			_, rightLiteral, err := m.SequenceStringValue(r1)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if leftLiteral && rightLiteral {
				middle, err := t.concatConstantStrings(l2, r1)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				l, err = t.concatTerm(l1, middle)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				r = r2
			}
		}
	}
	key := [2]ast.ASTHandle{l, r}
	if expression, found := t.concatCache[key]; found {
		return expression, nil
	}
	expression, err := m.MkSequenceConcat(l, r)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = t.retain(expression); err != nil {
		return ast.ASTHandle{}, err
	}
	t.concatCache[key] = expression
	return expression, t.assertFlattenedConcatLength(expression)
}

func (t *SMTStringTheory) assertFlattenedConcatLength(expression ast.ASTHandle) (result error) {
	m := t.state.manager
	var lengths []ast.ASTHandle
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
	}()
	work := []ast.ASTHandle{expression}
	for len(work) != 0 {
		last := len(work) - 1
		current := work[last]
		work = work[:last]
		left, right, concat, err := m.SequenceConcatParts(current)
		if err != nil {
			return err
		}
		if concat {
			work = append(work, right, left)
			continue
		}
		length, err := m.MkSequenceLength(current)
		if err != nil {
			return err
		}
		ref, err := m.NewRef(length)
		if err != nil {
			return err
		}
		refs = append(refs, ref)
		lengths = append(lengths, length)
	}
	length, err := m.MkSequenceLength(expression)
	if err != nil {
		return err
	}
	ref, err := m.NewRef(length)
	if err != nil {
		return err
	}
	refs = append(refs, ref)
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return err
	}
	sum, err := m.MkFamilyApp(family, 6, nil, lengths)
	if err != nil {
		return err
	}
	equality, err := m.MkEq(length, sum)
	if err != nil {
		return err
	}
	return t.assertAxiom(equality)
}
