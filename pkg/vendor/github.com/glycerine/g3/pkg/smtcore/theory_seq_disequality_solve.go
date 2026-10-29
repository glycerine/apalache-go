package smtcore

// Source: src/smt/theory_seq.cpp solve_nqs and solve_ne retained word
// expansion, equality reduction and conflict case.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTSequenceTheory) checkReducedDisequations() (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	progress := false
	for index := 0; !state.Inconsistent() && index < t.disequations.Len(); {
		remove, revisit, changed, err := t.reduceDisequation(index)
		if err != nil {
			return false, err
		}
		progress = progress || changed
		if remove {
			if err = t.disequations.EraseSwapBack(index); err != nil {
				return false, err
			}
			continue
		}
		if revisit {
			continue
		}
		index++
	}
	return progress || state.Inconsistent(), nil
}

func (t *SMTSequenceTheory) reduceDisequation(index int) (remove, revisit, progress bool, result error) {
	state, err := t.State()
	if err != nil {
		return false, false, false, err
	}
	originalLeft, originalRight, leftCases, rightCases, literals, dependency, err := t.disequations.At(index)
	if err != nil {
		return false, false, false, err
	}
	for _, literal := range literals {
		value, err := state.Assignment(literal)
		if err != nil {
			return false, false, false, err
		}
		if value == lbool.False {
			return true, false, false, nil
		}
	}
	updated := false
	var newLeft, newRight [][]ast.ASTHandle
	newLiterals := append([]SMTLiteral(nil), literals...)
	for i := range leftCases {
		left, updatedDependency, err := t.expandWord(leftCases[i], dependency)
		if err != nil {
			return false, false, false, err
		}
		right, updatedDependency, err := t.expandWord(rightCases[i], updatedDependency)
		if err != nil {
			return false, false, false, err
		}
		changed := !smtSequenceSameParts(left, leftCases[i]) || !smtSequenceSameParts(right, rightCases[i])
		parts := left
		if len(parts) == 0 {
			parts = right
		}
		sort, err := state.manager.GetSort(originalLeft)
		if len(parts) != 0 {
			sort, err = state.manager.GetSort(parts[0])
		}
		if err != nil {
			return false, false, false, err
		}
		left, right, updatedDependency, boundaryChanged, unequal, err := t.cancelWordBoundaries(left, right, updatedDependency)
		if err != nil {
			return false, false, false, err
		}
		if unequal {
			return true, false, progress, nil
		}
		changed = changed || boundaryChanged
		leftWord, err := t.makeWord(left, sort)
		if err != nil {
			return false, false, false, err
		}
		rightWord, err := t.makeWord(right, sort)
		if err != nil {
			return false, false, false, err
		}
		rw, err := rewriter.NewSequenceRewriter(state.manager)
		if err != nil {
			return false, false, false, err
		}
		reduction, err := rw.ReduceEquality(leftWord, rightWord)
		if err != nil {
			return false, false, false, err
		}
		defer func() { result = errors.Join(result, reduction.Close()) }()
		if reduction.Inconsistent {
			return true, false, progress, nil
		}
		left, right = reduction.Left, reduction.Right
		changed = changed || reduction.Changed
		updated = updated || changed
		dependency = updatedDependency
		if len(left) != 0 || len(right) != 0 {
			newLeft = append(newLeft, left)
			newRight = append(newRight, right)
		}
		for j, pairLeft := range reduction.EqualitiesLeft {
			pair := [2]ast.ASTHandle{pairLeft, reduction.EqualitiesRight[j]}
			sort, err := state.manager.GetSort(pair[0])
			if err != nil {
				return false, false, false, err
			}
			sequence, regex, err := t.sequenceSort(sort)
			if err != nil {
				return false, false, false, err
			}
			if sequence || regex {
				newLeft = append(newLeft, []ast.ASTHandle{pair[0]})
				newRight = append(newRight, []ast.ASTHandle{pair[1]})
				continue
			}
			if pair[0] == pair[1] {
				continue
			}
			equality, err := state.manager.MkEq(pair[0], pair[1])
			if err != nil {
				return false, false, false, err
			}
			if err = state.Internalize(equality, true); err != nil {
				return false, false, false, err
			}
			literal, err := state.Literal(equality)
			if err != nil {
				return false, false, false, err
			}
			if err = state.markRelevant(equality); err != nil {
				return false, false, false, err
			}
			newLiterals = append(newLiterals, literal)
			value, err := state.Assignment(literal)
			if err != nil {
				return false, false, false, err
			}
			if value == lbool.False {
				return true, false, progress, nil
			}
			if value == lbool.Undef {
				progress = true
				state.theoryProgress++
			}
		}
	}
	undefined := 0
	lastUndefined := SMTNullLiteral
	assertedDependency := dependency
	for _, literal := range newLiterals {
		value, err := state.Assignment(literal)
		if err != nil {
			return false, false, false, err
		}
		if value == lbool.False {
			return true, false, progress, nil
		}
		if value == lbool.Undef {
			undefined++
			lastUndefined = literal
		} else {
			assertedDependency = t.dependencies.Join(assertedDependency, t.dependencies.Leaf(smtSequenceAssumption{literal: literal}))
		}
	}
	if len(newLeft) == 0 {
		if undefined == 0 {
			if updated {
				explained, explanation, err := t.explainWordEquality(originalLeft, originalRight, nil)
				if err != nil {
					return false, false, false, err
				}
				if explained {
					literal := t.equalityLiteral(originalLeft, originalRight)
					if literal != SMTNullLiteral {
						value, err := state.Assignment(literal)
						if err != nil {
							return false, false, false, err
						}
						if value == lbool.False {
							assertedDependency = t.dependencies.Join(explanation, t.dependencies.Leaf(smtSequenceAssumption{literal: literal.Negated()}))
						}
					}
				}
			}
			err := t.setDependencyConflict(assertedDependency)
			return true, false, state.Inconsistent(), err
		}
		if undefined == 1 {
			propagated, err := t.propagateSequenceLiteral(lastUndefined.Negated(), assertedDependency)
			return true, false, progress || propagated, err
		}
	}
	if updated {
		if err = t.disequations.Set(index, originalLeft, originalRight, newLeft, newRight, newLiterals, dependency); err != nil {
			return false, false, false, err
		}
	}
	// Retain the existing explained residual-equality propagation in addition
	// to the source reduced-case record. The original record is now replaced.
	if len(newLeft) == 1 && len(newLiterals) == 0 {
		sort, err := state.manager.GetSort(originalLeft)
		if err != nil {
			return false, false, false, err
		}
		left, err := t.makeWord(newLeft[0], sort)
		if err != nil {
			return false, false, false, err
		}
		right, err := t.makeWord(newRight[0], sort)
		if err != nil {
			return false, false, false, err
		}
		original, err := state.manager.MkEq(originalLeft, originalRight)
		if err != nil {
			return false, false, false, err
		}
		equality, err := state.manager.MkEq(left, right)
		if err != nil {
			return false, false, false, err
		}
		if equality != original {
			if err = state.Internalize(equality, true); err != nil {
				return false, false, false, err
			}
			literal, err := state.Literal(equality)
			if err != nil {
				return false, false, false, err
			}
			propagated, err := t.propagateSequenceLiteral(literal.Negated(), dependency)
			return false, updated, progress || propagated, err
		}
	}
	return false, updated, progress, nil
}

func smtSequenceSameParts(first, second []ast.ASTHandle) bool {
	if len(first) != len(second) {
		return false
	}
	for i := range first {
		if first[i] != second[i] {
			return false
		}
	}
	return true
}

// Source mk_cell/unfold/explain_eq keeps parent boundaries so a common
// subexpression can be matched without using its irrelevant expansion.
type smtSequenceExplanationCell struct {
	parent     *smtSequenceExplanationCell
	expression ast.ASTHandle
	dependency *smtSequenceDependency
	last       int
}

func (t *SMTSequenceTheory) unfoldSequenceExplanation(cell *smtSequenceExplanationCell, leaves []*smtSequenceExplanationCell, active map[ast.ASTHandle]bool) ([]*smtSequenceExplanationCell, error) {
	if active[cell.expression] {
		return nil, ast.ErrASTDomain
	}
	active[cell.expression] = true
	defer delete(active, cell.expression)
	value, dependency, found, err := t.solutions.FindOne(cell.expression, nil)
	if err != nil {
		return nil, err
	}
	if found {
		child := &smtSequenceExplanationCell{parent: cell, expression: value, dependency: t.dependencies.Join(dependency, cell.dependency)}
		leaves, err = t.unfoldSequenceExplanation(child, leaves, active)
	} else {
		state, stateErr := t.State()
		if stateErr != nil {
			return nil, stateErr
		}
		left, right, concat, partsErr := state.manager.SequenceConcatParts(cell.expression)
		if partsErr != nil {
			return nil, partsErr
		}
		if concat {
			leaves, err = t.unfoldSequenceExplanation(&smtSequenceExplanationCell{parent: cell, expression: left, dependency: cell.dependency}, leaves, active)
			if err == nil {
				leaves, err = t.unfoldSequenceExplanation(&smtSequenceExplanationCell{expression: right}, leaves, active)
			}
		} else {
			leaves = append(leaves, cell)
		}
	}
	if err != nil {
		return nil, err
	}
	cell.last = len(leaves) - 1
	return leaves, nil
}

func (t *SMTSequenceTheory) explainWordEquality(left, right ast.ASTHandle, dependency *smtSequenceDependency) (bool, *smtSequenceDependency, error) {
	state, err := t.State()
	if err != nil {
		return false, dependency, err
	}
	if left == right {
		return true, dependency, nil
	}
	first, err := t.unfoldSequenceExplanation(&smtSequenceExplanationCell{expression: left}, nil, make(map[ast.ASTHandle]bool))
	if err != nil {
		return false, dependency, err
	}
	second, err := t.unfoldSequenceExplanation(&smtSequenceExplanationCell{expression: right}, nil, make(map[ast.ASTHandle]bool))
	if err != nil {
		return false, dependency, err
	}
	i, j := 0, 0
	for i < len(first) || j < len(second) {
		if i == len(first) || j == len(second) {
			remaining, index := first, i
			if i == len(first) {
				remaining, index = second, j
			}
			for index < len(remaining) {
				empty, err := state.manager.IsSequenceEmpty(remaining[index].expression)
				if err != nil || !empty {
					return false, dependency, err
				}
				dependency = t.dependencies.Join(dependency, remaining[index].dependency)
				index++
			}
			return true, dependency, nil
		}
		one, two := first[i], second[j]
		if one.expression == two.expression {
			if one.parent != nil && two.parent != nil && one.parent.expression == two.parent.expression {
				one, two = one.parent, two.parent
				first[one.last], second[two.last] = one, two
				i, j = one.last, two.last
			} else {
				dependency = t.dependencies.Join(dependency, one.dependency)
				dependency = t.dependencies.Join(dependency, two.dependency)
				i++
				j++
			}
			continue
		}
		empty, err := state.manager.IsSequenceEmpty(one.expression)
		if err != nil {
			return false, dependency, err
		}
		if empty {
			dependency = t.dependencies.Join(dependency, one.dependency)
			i++
			continue
		}
		empty, err = state.manager.IsSequenceEmpty(two.expression)
		if err != nil {
			return false, dependency, err
		}
		if empty {
			dependency = t.dependencies.Join(dependency, two.dependency)
			j++
			continue
		}
		a, unitA, err := state.manager.SequenceUnitArgument(one.expression)
		if err != nil {
			return false, dependency, err
		}
		b, unitB, err := state.manager.SequenceUnitArgument(two.expression)
		if err != nil || !unitA || !unitB {
			return false, dependency, err
		}
		// Keep the word-to-unit premises as well as the recursive element
		// premises. The native unit branch omits these cell dependencies.
		dependency = t.dependencies.Join(dependency, one.dependency)
		dependency = t.dependencies.Join(dependency, two.dependency)
		explained, updated, err := t.explainWordEquality(a, b, dependency)
		dependency = updated
		if err != nil || !explained {
			return false, dependency, err
		}
		i++
		j++
	}
	return true, dependency, nil
}

// Source propagate_lit retains both literal and enode-equality antecedents.
// A cancellation under established equalities is not an unconditional lemma.
func (t *SMTSequenceTheory) propagateSequenceLiteral(literal SMTLiteral, dependency *smtSequenceDependency) (bool, error) {
	state, err := t.State()
	if err != nil {
		return false, err
	}
	value, err := state.Assignment(literal)
	if err != nil || value == lbool.True {
		return false, err
	}
	if literal == SMTFalseLiteral {
		err := t.setDependencyConflict(dependency)
		return state.Inconsistent(), err
	}
	expression, err := state.BoolExpression(literal.Var())
	if err != nil {
		return false, err
	}
	if err = state.markRelevant(expression); err != nil {
		return false, err
	}
	literals, equalities := t.dependencyEvidence(dependency)
	for _, antecedent := range literals {
		assigned, err := state.Assignment(antecedent)
		if err != nil || assigned != lbool.True {
			return false, err
		}
	}
	owner, err := NewSMTExtTheoryPropagationJustification(state.manager, t.FamilyID(), literals, equalities, literal, nil)
	if err != nil {
		return false, err
	}
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		return false, errors.Join(err, owner.Release(state.manager))
	}
	state.ownedJustifications = append(state.ownedJustifications, owner)
	if err = state.AssignLiteral(literal, wrapped); err != nil {
		return false, err
	}
	state.theoryProgress++
	return true, nil
}
