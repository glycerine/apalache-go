package smtcore

// Source: src/smt/theory_dense_diff_logic_def.h:223-245 and
// src/smt/arith_eq_adapter.cpp:114-227.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTDenseDiffTheory) forcedDifferenceEquality(left, right ast.ASTHandle) (bool, error) {
	if _, numeral, err := t.syntax.numeral(right); err != nil || !numeral {
		return false, err
	}
	op, arithmetic, err := t.syntax.operation(left)
	if err != nil || !arithmetic || op != smtArithOpAdd {
		return false, err
	}
	count, err := t.state.manager.GetNumArgs(left)
	if err != nil || count != 2 {
		return false, err
	}
	negative, err := t.state.manager.GetArg(left, 1)
	if err != nil {
		return false, err
	}
	op, arithmetic, err = t.syntax.operation(negative)
	if err != nil || !arithmetic || op != smtArithOpMul {
		return false, err
	}
	count, err = t.state.manager.GetNumArgs(negative)
	if err != nil || count != 2 {
		return false, err
	}
	coefficient, err := t.state.manager.GetArg(negative, 0)
	if err != nil {
		return false, err
	}
	value, numeral, err := t.syntax.numeral(coefficient)
	return err == nil && numeral && value.Cmp(big.NewRat(-1, 1)) == 0, err
}

func (t *SMTDenseDiffTheory) denseEqualityOperandsAttached(left, right ast.ASTHandle) bool {
	for _, expression := range []ast.ASTHandle{left, right} {
		node := t.state.context.enodes[expression]
		if node == nil || !t.IsAttachedToVariable(node) {
			return false
		}
	}
	return true
}

func (t *SMTDenseDiffTheory) denseEqualityBounds(left, right ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, error) {
	m := t.state.manager
	if _, numeral, err := t.syntax.numeral(left); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	} else if numeral {
		left, right = right, left
	}
	if _, numeral, err := t.syntax.numeral(right); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	} else if numeral {
		lower, err := m.MkFamilyApp(t.FamilyID(), smtArithOpLE, nil, []ast.ASTHandle{left, right})
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		upper, err := m.MkFamilyApp(t.FamilyID(), smtArithOpGE, nil, []ast.ASTHandle{left, right})
		return lower, upper, err
	}
	sort, err := m.GetSort(left)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	integer, err := t.syntax.integerSort(sort)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	minusOne, err := m.ArithmeticNumeralRat(big.NewRat(-1, 1), integer)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	negativeRight, err := m.MkFamilyApp(t.FamilyID(), smtArithOpMul, nil, []ast.ASTHandle{minusOne, right})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	difference, err := m.MkFamilyApp(t.FamilyID(), smtArithOpAdd, nil, []ast.ASTHandle{left, negativeRight})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	zero, err := m.ArithmeticNumeralRat(new(big.Rat), integer)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	lower, err := m.MkFamilyApp(t.FamilyID(), smtArithOpLE, nil, []ast.ASTHandle{difference, zero})
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	upper, err := m.MkFamilyApp(t.FamilyID(), smtArithOpGE, nil, []ast.ASTHandle{difference, zero})
	return lower, upper, err
}

func (t *SMTDenseDiffTheory) InternalizeEquality(expression ast.ASTHandle, variable int32) error {
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
	return t.installDenseEqualityAxioms(expression, variable, left, right, false)
}

func (t *SMTDenseDiffTheory) installDenseEqualityAxioms(expression ast.ASTHandle, variable int32, left, right ast.ASTHandle, relationEvent bool) (result error) {
	state := t.state
	forced, err := t.forcedDifferenceEquality(left, right)
	if err != nil {
		return err
	}
	if !relationEvent && !forced && (!t.config.EagerEqAxioms || !t.denseEqualityOperandsAttached(left, right)) {
		return nil
	}
	leftValue, err := state.manager.IsValue(left)
	if err != nil {
		return err
	}
	rightValue, err := state.manager.IsValue(right)
	if err != nil {
		return err
	}
	if left == right || leftValue && rightValue {
		return nil
	}
	leftNode := state.context.enodes[left]
	rightNode := state.context.enodes[right]
	if leftNode == nil || rightNode == nil {
		return ast.ErrASTDomain
	}
	pair, err := t.adapter.pair(leftNode, rightNode)
	if err != nil {
		return err
	}
	if _, found := t.adapter.processed[pair]; found {
		return nil
	}
	t.adapter.processed[pair] = struct{}{}
	t.adapter.trail = append(t.adapter.trail, pair)
	t.adapter.numAxioms++
	committed := false
	defer func() {
		if committed || result == nil {
			return
		}
		delete(t.adapter.processed, pair)
		t.adapter.trail = t.adapter.trail[:len(t.adapter.trail)-1]
		t.adapter.numAxioms--
	}()
	lower, upper, err := t.denseEqualityBounds(left, right)
	if err != nil {
		return err
	}
	for _, bound := range []ast.ASTHandle{lower, upper} {
		if err = state.Internalize(bound, true); err != nil {
			return err
		}
	}
	lowerLiteral, err := state.Literal(lower)
	if err != nil {
		return err
	}
	upperLiteral, err := state.Literal(upper)
	if err != nil {
		return err
	}
	equality := NewSMTLiteral(variable, false)
	data, err := state.BooleanData(variable)
	if err != nil {
		return err
	}
	if data.TryTrueFirst() {
		if _, err = state.setScopedTrueFirst(lowerLiteral.Var()); err != nil {
			return err
		}
		if _, err = state.setScopedTrueFirst(upperLiteral.Var()); err != nil {
			return err
		}
	}
	proofs, err := state.manager.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs && len(t.adapter.hint) == 0 {
		symbol, err := state.manager.Symbols.Intern("triangle-eq")
		if err != nil {
			return err
		}
		parameter, err := state.manager.NewParameterSymbol(symbol)
		if err != nil {
			return err
		}
		t.adapter.hint = append(t.adapter.hint, parameter)
	}
	for _, clause := range [][]SMTLiteral{
		{equality.Negated(), lowerLiteral},
		{equality.Negated(), upperLiteral},
		{equality, lowerLiteral.Negated(), upperLiteral.Negated()},
	} {
		if err = state.addTheoryAxiomWithParameters(t.FamilyID(), clause, t.adapter.hint); err != nil {
			return err
		}
	}
	if t.config.AddBinaryBounds {
		if err = state.addTheoryAxiomWithParameters(t.FamilyID(), []SMTLiteral{lowerLiteral, upperLiteral}, t.adapter.hint); err != nil {
			return err
		}
	}
	if err = t.adapter.AddRelevancy(left, right, expression, lower, upper); err != nil {
		return err
	}
	if !t.adapter.lazy && state.searching && state.scopeLevel > state.searchBaseLevel &&
		leftNode.InternalScopeLevel() <= state.searchBaseLevel && rightNode.InternalScopeLevel() <= state.searchBaseLevel {
		t.adapter.restart = append(t.adapter.restart, pair)
	}
	committed = true
	return nil
}

func (t *SMTDenseDiffTheory) denseRelationAxioms(left, right SMTTheoryVar) error {
	state, err := t.State()
	if err != nil {
		return err
	}
	leftNode, err := t.Enode(left)
	if err != nil {
		return err
	}
	rightNode, err := t.Enode(right)
	if err != nil {
		return err
	}
	equality, err := state.manager.MkEq(leftNode.Owner(), rightNode.Owner())
	if err != nil || state.manager.IsFalse(equality) {
		return err
	}
	if err = state.Internalize(equality, true); err != nil {
		return err
	}
	variable, found := state.BoolVar(equality)
	if !found {
		return ast.ErrASTDomain
	}
	return t.installDenseEqualityAxioms(equality, variable, leftNode.Owner(), rightNode.Owner(), true)
}

func (t *SMTDenseDiffTheory) NewEquality(left, right SMTTheoryVar) error {
	return t.denseRelationAxioms(left, right)
}

func (t *SMTDenseDiffTheory) NewDisequality(left, right SMTTheoryVar) error {
	return t.denseRelationAxioms(left, right)
}

func (t *SMTDenseDiffTheory) Restart() error {
	state, err := t.State()
	if err != nil {
		return err
	}
	pairs := append([]smtArithmeticEqualityPair(nil), t.adapter.restart...)
	t.adapter.restart = nil
	for _, pair := range pairs {
		if state.inconsistent {
			break
		}
		left := state.context.enodes[pair.first]
		right := state.context.enodes[pair.second]
		if left == nil || right == nil || !t.IsAttachedToVariable(left) || !t.IsAttachedToVariable(right) {
			return ast.ErrASTDomain
		}
		if err = t.denseRelationAxioms(left.TheoryVar(t.FamilyID()), right.TheoryVar(t.FamilyID())); err != nil {
			return err
		}
	}
	return nil
}
