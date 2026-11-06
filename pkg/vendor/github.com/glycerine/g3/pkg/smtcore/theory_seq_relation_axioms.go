package smtcore

// Source: src/smt/theory_seq.cpp assign_eh positive prefix, suffix and
// containment witnesses plus propagate_non_empty guards.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTSequenceTheory) relationSkolem(name string, arguments []ast.ASTHandle, sort ast.ASTHandle) (ast.ASTHandle, error) {
	state, err := t.State()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	symbol, err := state.manager.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return state.manager.MkSequenceSkolem(symbol, arguments, sort)
}

func (t *SMTSequenceTheory) addNegativePrefixStep(expression ast.ASTHandle) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	boundary, container, matched, err := state.manager.SequenceBinaryOperationArguments(expression, 3)
	if err != nil || !matched {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	sort, err := state.manager.GetSort(boundary)
	if err != nil {
		return err
	}
	elementSort, sequence, err := state.manager.SequenceSortElement(sort)
	if err != nil || !sequence {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	empty, err := state.manager.MkSequenceEmpty(sort)
	if err != nil {
		return err
	}
	containerEmpty, err := state.manager.MkEq(container, empty)
	if err != nil {
		return err
	}
	literal, err := state.manager.MkEq(container, empty)
	if err != nil {
		return err
	}
	if err = state.Internalize(literal, false); err != nil {
		return err
	}
	emptyLiteral, err := state.Literal(literal)
	if err != nil {
		return err
	}
	assignment, err := state.Assignment(emptyLiteral)
	if err != nil || assignment == lbool.True {
		return err
	}
	if assignment == lbool.Undef {
		wasRelevant := state.isRelevant(literal)
		if err = state.markRelevant(literal); err != nil {
			return err
		}
		data, err := state.BooleanData(emptyLiteral.Var())
		if err != nil {
			return err
		}
		phase := !emptyLiteral.Sign()
		if !wasRelevant || !data.PhaseAvailable || data.Phase != phase {
			state.theoryProgress++
		}
		data.PhaseAvailable, data.Phase = true, phase
		return nil
	}
	if err = t.addSequenceCoherenceAxiom(boundary); err != nil {
		return err
	}
	if err = t.addSequenceCoherenceAxiom(container); err != nil {
		return err
	}
	zero, err := t.integer(0)
	if err != nil {
		return err
	}
	boundaryHead, err := t.relationSkolem("seq.nth", []ast.ASTHandle{boundary, zero}, elementSort)
	if err != nil {
		return err
	}
	containerHead, err := t.relationSkolem("seq.nth", []ast.ASTHandle{container, zero}, elementSort)
	if err != nil {
		return err
	}
	boundaryTail, err := t.relationSkolem("seq.tail", []ast.ASTHandle{boundary, zero}, sort)
	if err != nil {
		return err
	}
	containerTail, err := t.relationSkolem("seq.tail", []ast.ASTHandle{container, zero}, sort)
	if err != nil {
		return err
	}
	headsEqual, err := state.manager.MkEq(boundaryHead, containerHead)
	if err != nil {
		return err
	}
	if err = state.Internalize(headsEqual, false); err != nil {
		return err
	}
	headLiteral, err := state.Literal(headsEqual)
	if err != nil {
		return err
	}
	headAssignment, err := state.Assignment(headLiteral)
	if err != nil || headAssignment == lbool.False {
		return err
	}
	if headAssignment == lbool.Undef {
		wasRelevant := state.isRelevant(headsEqual)
		if err = state.markRelevant(headsEqual); err != nil {
			return err
		}
		data, err := state.BooleanData(headLiteral.Var())
		if err != nil {
			return err
		}
		phase := headLiteral.Sign()
		if !wasRelevant || !data.PhaseAvailable || data.Phase != phase {
			state.theoryProgress++
		}
		data.PhaseAvailable, data.Phase = true, phase
		return nil
	}
	tailPrefix, err := state.manager.MkSequencePrefix(boundaryTail, containerTail)
	if err != nil {
		return err
	}
	notTailPrefix, err := state.manager.MkNot(tailPrefix)
	if err != nil {
		return err
	}
	notPrefix, err := state.manager.MkNot(expression)
	if err != nil {
		return err
	}
	nonempty, err := state.manager.MkNot(containerEmpty)
	if err != nil {
		return err
	}
	if err = t.pushSequenceImplication([]ast.ASTHandle{notPrefix, nonempty, headsEqual}, notTailPrefix); err != nil {
		return err
	}
	t.trackInternalTerms(boundaryHead, containerHead, boundaryTail, containerTail, tailPrefix)
	return nil
}

func (t *SMTSequenceTheory) addRelationAxioms(expression ast.ASTHandle, operation int32, positive bool) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	left, err := state.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	right, err := state.manager.GetArg(expression, 1)
	if err != nil {
		return err
	}
	sort, err := state.manager.GetSort(left)
	if err != nil {
		return err
	}
	if positive {
		arguments := []ast.ASTHandle{left, right}
		var definition ast.ASTHandle
		switch operation {
		case 3:
			remainder, makeErr := t.relationSkolem("seq.p.suffix", arguments, sort)
			if makeErr != nil {
				return makeErr
			}
			word, makeErr := state.manager.MkSequenceConcat(left, remainder)
			if makeErr != nil {
				return makeErr
			}
			definition, err = state.manager.MkEq(word, right)
		case 4:
			remainder, makeErr := t.relationSkolem("seq.s.prefix", arguments, sort)
			if makeErr != nil {
				return makeErr
			}
			word, makeErr := state.manager.MkSequenceConcat(remainder, left)
			if makeErr != nil {
				return makeErr
			}
			definition, err = state.manager.MkEq(word, right)
		case 5:
			prefix, makeErr := t.relationSkolem("seq.idx.left", arguments, sort)
			if makeErr != nil {
				return makeErr
			}
			suffix, makeErr := t.relationSkolem("seq.idx.right", arguments, sort)
			if makeErr != nil {
				return makeErr
			}
			tail, makeErr := state.manager.MkSequenceConcat(right, suffix)
			if makeErr != nil {
				return makeErr
			}
			word, makeErr := state.manager.MkSequenceConcat(prefix, tail)
			if makeErr != nil {
				return makeErr
			}
			definition, err = state.manager.MkEq(word, left)
		default:
			return ast.ErrASTDomain
		}
		if err != nil {
			return err
		}
		witness, err := state.manager.MkImplies(expression, definition)
		if err != nil {
			return err
		}
		return t.axioms.Push(witness)

	}

	nonemptyArgument := left
	if operation == 5 {
		nonemptyArgument = right
	}
	empty, err := state.manager.MkSequenceEmpty(sort)
	if err != nil {
		return err
	}
	emptyEquality, err := state.manager.MkEq(nonemptyArgument, empty)
	if err != nil {
		return err
	}
	nonempty, err := state.manager.MkNot(emptyEquality)
	if err != nil {
		return err
	}
	notRelation, err := state.manager.MkNot(expression)
	if err != nil {
		return err
	}
	guard, err := state.manager.MkImplies(notRelation, nonempty)
	if err != nil {
		return err
	}
	if err = t.axioms.Push(guard); err != nil {
		return err
	}
	if operation == 3 {
		return nil
	}
	if operation == 4 {
		return t.addNegativeSuffixWitness(expression, left, right)
	}
	return nil
}
