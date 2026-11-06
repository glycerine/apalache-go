package smtcore

// Source: src/smt/theory_arith_core.h mk_to_int_axiom, internalize_to_int,
// mk_is_int_axiom and internalize_is_int.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (t *SMTLinearArithmeticTheory) assertArithmeticAxiom(formula ast.ASTHandle) (result error) {
	s := t.state
	ref, err := s.manager.NewRef(formula)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, s.manager.DelRef(&ref)) }()
	if err = s.Internalize(formula, true); err != nil {
		return err
	}
	literal, err := s.Literal(formula)
	if err != nil {
		return err
	}
	if err = s.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal}); err != nil {
		return err
	}
	if literal.Var() == SMTTrueBoolVar {
		return nil
	}
	atom, err := s.BoolExpression(literal.Var())
	if err != nil {
		return err
	}
	return s.markRelevant(atom)
}

func (t *SMTLinearArithmeticTheory) installToIntAxioms(term ast.ASTHandle) (result error) {
	if t.castTerms[term] {
		return nil
	}
	t.castTerms[term] = true
	t.castTermTrail = append(t.castTermTrail, term)
	s := t.state
	m := s.manager
	x, err := m.GetArg(term, 0)
	if err != nil {
		return err
	}
	cast, own, err := t.operation(x)
	if err != nil {
		return err
	}
	if own && cast == smtArithOpToReal {
		original, err := m.GetArg(x, 0)
		if err != nil {
			return err
		}
		eq, err := m.MkEq(original, term)
		if err != nil {
			return err
		}
		return t.assertArithmeticAxiom(eq)
	}
	realTerm, err := m.MkFamilyApp(t.FamilyID(), smtArithOpToReal, nil, []ast.ASTHandle{term})
	if err != nil {
		return err
	}
	negative, err := m.MkFamilyApp(t.FamilyID(), smtArithOpUMinus, nil, []ast.ASTHandle{realTerm})
	if err != nil {
		return err
	}
	difference, err := m.MkArithmeticAdd(x, negative)
	if err != nil {
		return err
	}
	diffRef, err := m.NewRef(difference)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&diffRef)) }()
	zero, err := m.ArithmeticRealNumeralText("0")
	if err != nil {
		return err
	}
	lower, err := m.MkFamilyApp(t.FamilyID(), smtArithOpGE, nil, []ast.ASTHandle{difference, zero})
	if err != nil {
		return err
	}
	lowerRef, err := m.NewRef(lower)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&lowerRef)) }()
	one, err := m.ArithmeticRealNumeralText("1")
	if err != nil {
		return err
	}
	upper, err := m.MkFamilyApp(t.FamilyID(), smtArithOpGE, nil, []ast.ASTHandle{difference, one})
	if err != nil {
		return err
	}
	upper, err = m.MkNot(upper)
	if err != nil {
		return err
	}
	upperRef, err := m.NewRef(upper)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, m.DelRef(&upperRef)) }()
	if err = t.assertArithmeticAxiom(lower); err != nil {
		return err
	}
	return t.assertArithmeticAxiom(upper)
}

func (t *SMTLinearArithmeticTheory) internalizeIsInt(expression ast.ASTHandle) (handled bool, result error) {
	s := t.state
	m := s.manager
	variable, found := s.BoolVar(expression)
	var err error
	if !found {
		variable, err = s.MkBoolVar(expression)
	}
	if err != nil {
		return false, err
	}
	if t.castPredicates[variable] {
		return true, nil
	}
	if err = s.SetVariableTheory(variable, t.FamilyID()); err != nil {
		return false, err
	}
	t.castPredicates[variable] = true
	t.castPredicateTrail = append(t.castPredicateTrail, variable)
	x, err := m.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	floor, err := m.MkFamilyApp(t.FamilyID(), smtArithOpToInt, nil, []ast.ASTHandle{x})
	if err != nil {
		return false, err
	}
	realFloor, err := m.MkFamilyApp(t.FamilyID(), smtArithOpToReal, nil, []ast.ASTHandle{floor})
	if err != nil {
		return false, err
	}
	eq, err := m.MkEq(realFloor, x)
	if err != nil {
		return false, err
	}
	ref, err := m.NewRef(eq)
	if err != nil {
		return false, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	if err = s.Internalize(eq, true); err != nil {
		return false, err
	}
	literal, err := s.Literal(eq)
	if err != nil {
		return false, err
	}
	predicate := NewSMTLiteral(variable, false)
	if err = s.addTheoryAxiom(t.FamilyID(), []SMTLiteral{predicate.Negated(), literal}); err != nil {
		return false, err
	}
	if err = s.addTheoryAxiom(t.FamilyID(), []SMTLiteral{literal.Negated(), predicate}); err != nil {
		return false, err
	}
	return true, nil
}
