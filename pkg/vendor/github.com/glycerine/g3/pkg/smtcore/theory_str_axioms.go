package smtcore

// Source: src/smt/theory_str.cpp assert_axiom, instantiate_basic_string_axioms,
// instantiate_concat_axiom and instantiate_str_eq_length_axiom.

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTStringTheory) assertAxiom(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil || expression.IsNull() || state.manager.IsTrue(expression) {
		return err
	}
	if err = state.Internalize(expression, false); err != nil {
		return err
	}
	literal, err := state.Literal(expression)
	if err != nil {
		return err
	}
	if err = state.markRelevant(expression); err != nil {
		return err
	}
	if err = state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal}); err != nil {
		return err
	}
	return t.retain(expression)
}

func (t *SMTStringTheory) instantiateBasicStringAxioms(node *SMTEnode) (result error) {
	state, err := t.State()
	if err != nil {
		return err
	}
	if node == nil || node.manager != state.manager {
		return ast.ErrASTDomain
	}
	m := state.manager
	sort, err := m.GetSort(node.Owner())
	if err != nil {
		return err
	}
	stringSort, err := m.IsStringSort(sort)
	if err != nil || !stringSort || node.InternalScopeLevel() > state.scopeLevel {
		return err
	}
	length, err := m.MkSequenceLength(node.Owner())
	if err != nil {
		return err
	}
	lengthRef, err := m.NewRef(length)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&lengthRef)) }()
	text, constant, err := m.SequenceStringValue(node.Owner())
	if err != nil {
		return err
	}
	if constant {
		number, err := m.ArithmeticIntegerNumeralText(strconv.Itoa(len(text)))
		if err != nil {
			return err
		}
		equality, err := m.MkEq(length, number)
		if err != nil {
			return err
		}
		// Source constant-length case publishes its unit directly, rather
		// than retaining this equality in assert_axiom's lifetime trail.
		if err = state.Internalize(equality, false); err != nil {
			return err
		}
		literal, err := state.Literal(equality)
		if err != nil {
			return err
		}
		if err = state.markRelevant(equality); err != nil {
			return err
		}
		return state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal})
	}
	zero, err := m.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return err
	}
	nonnegative, err := m.MkArithmeticGE(length, zero)
	if err != nil {
		return err
	}
	if err = t.assertAxiom(nonnegative); err != nil {
		return err
	}
	lengthZero, err := m.MkEq(length, zero)
	if err != nil {
		return err
	}
	zeroRef, err := m.NewRef(lengthZero)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&zeroRef)) }()
	empty, err := t.stringLiteral("")
	if err != nil {
		return err
	}
	isEmpty, err := m.MkEq(node.Owner(), empty)
	if err != nil {
		return err
	}
	equivalence, err := m.MkEq(lengthZero, isEmpty)
	if err != nil {
		return err
	}
	return t.assertAxiom(equivalence)
}

func (t *SMTStringTheory) instantiateConcatAxiom(node *SMTEnode) (result error) {
	m := t.state.manager
	left, right, concat, err := m.SequenceConcatParts(node.Owner())
	if err != nil || !concat {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	var lengths [3]ast.ASTHandle
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
	}()
	for i, expression := range []ast.ASTHandle{node.Owner(), left, right} {
		lengths[i], err = m.MkSequenceLength(expression)
		if err != nil {
			return err
		}
		ref, err := m.NewRef(lengths[i])
		if err != nil {
			return err
		}
		refs = append(refs, ref)
	}
	sum, err := m.MkArithmeticAdd(lengths[1], lengths[2])
	if err != nil {
		return err
	}
	equality, err := m.MkEq(lengths[0], sum)
	if err != nil {
		return err
	}
	return t.assertAxiom(equality)
}

func (t *SMTStringTheory) instantiateStringEqualityLengthAxiom(left, right *SMTEnode) (result error) {
	return t.assertStringLengthEquality(left.Owner(), right.Owner())
}

func (t *SMTStringTheory) assertStringLengthEquality(left, right ast.ASTHandle) (result error) {
	m := t.state.manager
	premise, err := m.MkEq(left, right)
	if err != nil {
		return err
	}
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
	}()
	ref, err := m.NewRef(premise)
	if err != nil {
		return err
	}
	refs = append(refs, ref)
	var lengths [2]ast.ASTHandle
	for i, expression := range []ast.ASTHandle{left, right} {
		lengths[i], err = m.MkSequenceLength(expression)
		if err != nil {
			return err
		}
		ref, err := m.NewRef(lengths[i])
		if err != nil {
			return err
		}
		refs = append(refs, ref)
	}
	conclusion, err := m.MkEq(lengths[0], lengths[1])
	if err != nil {
		return err
	}
	notPremise, err := m.MkNot(premise)
	if err != nil {
		return err
	}
	axiom, err := m.MkOr(notPremise, conclusion)
	if err != nil {
		return err
	}
	return t.assertAxiom(axiom)
}

// Source try_eval_concat is syntactic evaluation, unlike mk_value_helper's
// private equivalence-class fallback. Model choices cannot justify an axiom.
func (t *SMTStringTheory) tryEvalConcat(node *SMTEnode) (result error) {
	m := t.state.manager
	left, right, concat, err := m.SequenceConcatParts(node.Owner())
	if err != nil || !concat {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return err
	}
	work := []ast.ASTHandle{right, left}
	var text strings.Builder
	for len(work) != 0 {
		last := len(work) - 1
		expression := work[last]
		work = work[:last]
		value, constant, err := m.SequenceStringValue(expression)
		if err != nil {
			return err
		}
		if constant {
			text.WriteString(value)
			continue
		}
		left, right, concat, err := m.SequenceConcatParts(expression)
		if err != nil || !concat {
			return err
		}
		work = append(work, right, left)
	}
	value, err := t.stringLiteral(text.String())
	if err != nil {
		return err
	}
	ref, err := m.NewRef(value)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	equality, err := m.MkEq(node.Owner(), value)
	if err != nil {
		return err
	}
	return t.assertAxiom(equality)
}

func (t *SMTStringTheory) CanPropagate() bool {
	return len(t.basicTodo) != 0 || len(t.concatTodo) != 0 || len(t.concatEvalTodo) != 0 || len(t.libraryTodo) != 0 || len(t.delayedSetup) != 0 || len(t.regexStage.persistedTodo) != 0 || (t.searchStarted && len(t.delayedAssertions) != 0)
}

func (t *SMTStringTheory) Propagate() error {
	if _, err := t.State(); err != nil {
		return err
	}
	// Native basic-string work can recursively internalize fresh constants.
	for t.CanPropagate() {
		for len(t.basicTodo) != 0 {
			before := len(t.basicTodo)
			snapshot := append([]*SMTEnode(nil), t.basicTodo...)
			for _, node := range snapshot {
				if err := t.instantiateBasicStringAxioms(node); err != nil {
					return err
				}
			}
			if len(t.basicTodo) <= before {
				t.basicTodo = nil
			}
		}
		for _, node := range t.concatTodo {
			if err := t.instantiateConcatAxiom(node); err != nil {
				return err
			}
		}
		t.concatTodo = nil
		for _, node := range t.concatEvalTodo {
			if err := t.tryEvalConcat(node); err != nil {
				return err
			}
		}
		t.concatEvalTodo = nil
		for len(t.libraryTodo) != 0 {
			before := len(t.libraryTodo)
			snapshot := append([]*SMTEnode(nil), t.libraryTodo...)
			for _, node := range snapshot {
				op, own, err := t.operation(node.Owner())
				if err != nil {
					return err
				}
				if !own {
					return ast.ErrASTDomain
				}
				switch op {
				case 6:
					err = t.instantiateSubstring(node)
				case 7:
					err = t.instantiateReplace(node)
				case 8:
					err = t.instantiateCharAt(node)
				case 10:
					err = t.instantiateIndex(node)
				case 3, 4:
					err = t.instantiateBoundaryRelation(node, op == 4)
				case 5:
					err = t.instantiateContains(node)
				case 12:
					err = t.instantiateRegexMembership(node)
				case 27:
					err = t.instantiateIntToString(node)
				case 28:
					err = t.instantiateStringToInt(node)
				default:
					return fmt.Errorf("string theory library operation %d is not ported", op)
				}
				if err != nil {
					return err
				}
			}
			if len(t.libraryTodo) <= before {
				t.libraryTodo = nil
			}
		}
		for _, expression := range t.delayedSetup {
			if err := t.state.Internalize(expression, false); err != nil {
				return err
			}
			if err := t.setUpAxioms(expression); err != nil {
				return err
			}
		}
		t.delayedSetup = nil
		if err := t.propagatePersistedStringAxioms(); err != nil {
			return err
		}
		if t.searchStarted {
			for _, ref := range t.delayedAssertions {
				expression, err := t.state.manager.RefGet(ref)
				if err != nil {
					return err
				}
				if err := t.assertAxiom(expression); err != nil {
					return err
				}
			}
			for i := range t.delayedAssertions {
				if err := t.state.manager.DelRef(&t.delayedAssertions[i]); err != nil {
					return err
				}
			}
			t.delayedAssertions = nil
		}
	}
	return nil
}
