package smtcore

// Source: src/smt/theory_arith_core.h new_eq_eh and eq_bound justifications.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
)

type smtArithmeticEqualityEndpoints struct {
	lower, upper *smtArithmeticEqualityBound
}

func smtArithmeticDependencyKey(dependency simplex.BoundDependency) [2]uint32 {
	return [2]uint32{uint32(dependency.Literal), dependency.Equality}
}

func smtArithmeticPushDependency(out *smtArithmeticAntecedents, dependency simplex.BoundDependency, coefficient *big.Rat, proofs bool, bounds []smtArithmeticEqualityEndpoints) error {
	if dependency.Equality == 0 {
		return out.pushLiteral(SMTLiteralFromIndex(dependency.Literal), coefficient, proofs)
	}
	if uint64(dependency.Equality) > uint64(len(bounds)) {
		return ast.ErrASTDomain
	}
	return bounds[dependency.Equality-1].lower.pushJustification(out, coefficient, proofs)
}

type smtArithmeticOwnEquality struct{ left, right SMTTheoryVar }

func (t *SMTLinearArithmeticTheory) replayOwnEqualities() error {
	for t.ownEqualityHead < len(t.ownEqualities) {
		entry := t.ownEqualities[t.ownEqualityHead]
		left, err := t.Enode(entry.left)
		if err != nil {
			return err
		}
		right, err := t.Enode(entry.right)
		if err != nil {
			return err
		}
		if left.root != right.root {
			return nil // The logical-context equality phase has not run yet.
		}
		if err = t.NewEquality(entry.left, entry.right); err != nil {
			return err
		}
		t.ownEqualityHead++
	}
	return nil
}

func (t *SMTLinearArithmeticTheory) queueCongruenceEqualityBounds(first, second SMTTheoryVar) error {
	if first == second {
		return nil
	}
	left, err := t.Enode(first)
	if err != nil {
		return err
	}
	right, err := t.Enode(second)
	if err != nil {
		return err
	}
	if left.root != right.root {
		return ast.ErrASTDomain
	}
	syntax := SMTIntegerDiffTheory{SMTTheoryBase: t.SMTTheoryBase}
	m := t.state.manager
	if _, numeral, err := syntax.numeral(left.owner); err != nil {
		return err
	} else if numeral {
		first, second = second, first
		left, right = right, left
	}
	value, numeral, err := syntax.numeral(right.owner)
	if err != nil {
		return err
	}
	variable := first
	if !numeral {
		leftID, err := m.GetASTID(left.owner)
		if err != nil {
			return err
		}
		rightID, err := m.GetASTID(right.owner)
		if err != nil {
			return err
		}
		if leftID > rightID {
			left, right = right, left
		}
		sort, err := m.GetSort(left.owner)
		if err != nil {
			return err
		}
		integer, err := syntax.integerSort(sort)
		if err != nil {
			return err
		}
		minusOne, err := m.ArithmeticNumeralRat(big.NewRat(-1, 1), integer)
		if err != nil {
			return err
		}
		negative, err := m.MkFamilyApp(t.FamilyID(), smtArithOpMul, nil, []ast.ASTHandle{minusOne, right.owner})
		if err != nil {
			return err
		}
		difference, err := m.MkFamilyApp(t.FamilyID(), smtArithOpAdd, nil, []ast.ASTHandle{left.owner, negative})
		if err != nil {
			return err
		}
		if err := t.state.Internalize(difference, false); err != nil {
			return err
		}
		if t.state.relevancy != nil {
			if err := t.state.relevancy.MarkRelevant(difference); err != nil {
				return err
			}
		} else if err := t.state.context.SetRelevant(difference, true); err != nil {
			return err
		}
		node := t.state.context.enodes[difference]
		if node == nil || !t.IsAttachedToVariable(node) {
			return ast.ErrASTDomain
		}
		variable = node.TheoryVar(t.FamilyID())
		value = new(big.Rat)
	}
	var native smtArithmeticValue
	native.rational.Set(value)
	lower, err := newSMTArithmeticEqualityBound(variable, &native, smtArithmeticLowerBound, left, right)
	if err != nil {
		return err
	}
	upper, err := newSMTArithmeticEqualityBound(variable, &native, smtArithmeticUpperBound, left, right)
	if err != nil {
		return err
	}
	t.equalityBounds = append(t.equalityBounds, smtArithmeticEqualityEndpoints{lower, upper})
	id := uint32(len(t.equalityBounds))
	for _, low := range []bool{true, false} {
		t.pendingBounds = append(t.pendingBounds, smtArithmeticReplayBound{column: t.simplexVariables[variable], lower: low, value: simplex.RationalInfinitesimal(value), explanation: &smtArithmeticBoundExplanation{dependencies: []simplex.BoundDependency{{Equality: id, Coefficient: *big.NewRat(1, 1)}}}})
	}
	t.state.theoryProgress++
	return nil
}

func (t *SMTLinearArithmeticTheory) setArithmeticConflict(antecedents *smtArithmeticAntecedents, parameters ...ast.ASTParameter) error {
	if len(antecedents.equalities) == 0 {
		return t.state.SetTheoryConflict(t.FamilyID(), antecedents.literals, parameters...)
	}
	if t.state.inconsistent {
		return nil
	}
	owner, err := NewSMTExtTheoryConflictJustification(t.state.manager, t.FamilyID(), antecedents.literals, antecedents.equalities, parameters)
	if err != nil {
		return err
	}
	wrapped, err := NewSMTBooleanJustification(owner)
	if err != nil {
		_ = owner.Release(t.state.manager)
		return err
	}
	t.state.ownedJustifications = append(t.state.ownedJustifications, owner)
	t.state.inconsistent = true
	t.state.conflict = wrapped
	t.state.conflictLiteral = SMTNullLiteral
	t.state.statistics.NumConflicts++
	return nil
}

func (t *SMTLinearArithmeticTheory) allArithmeticAntecedents() (smtArithmeticAntecedents, error) {
	var result smtArithmeticAntecedents
	for _, assertion := range t.assertions {
		result.literals = append(result.literals, smtLinearAssertionLiteral(&t.atoms[assertion.atom], assertion))
	}
	for _, endpoints := range t.equalityBounds {
		if err := endpoints.lower.pushJustification(&result, big.NewRat(1, 1), false); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (t *SMTLinearArithmeticTheory) arithmeticDependencyLiterals(dependency simplex.BoundDependency) ([]SMTLiteral, error) {
	if dependency.Equality == 0 {
		return []SMTLiteral{SMTLiteralFromIndex(dependency.Literal)}, nil
	}
	if uint64(dependency.Equality) > uint64(len(t.equalityBounds)) {
		return nil, ast.ErrASTDomain
	}
	bound := t.equalityBounds[dependency.Equality-1].lower
	return NewSMTAntecedentExpander().ExpandEquality(bound.left, bound.right)
}

func (t *SMTLinearArithmeticTheory) setArithmeticFallbackConflict(literals []SMTLiteral) error {
	antecedents, err := t.allArithmeticAntecedents()
	if err != nil {
		return err
	}
	antecedents.literals = literals
	return t.setArithmeticConflict(&antecedents)
}
