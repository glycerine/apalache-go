package smtcore

// Source: src/smt/theory_arith_nl.h mk_gb_monomial, add_row_to_gb,
// add_monomial_def_to_gb and is_inconsistent(equation).

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
)

// The concrete Grobner solver retains AST dependency graphs. Their leaves are
// the actual assigned arithmetic literals, including negative literals.
type smtGrobnerDependencies struct {
	theory *SMTLinearArithmeticTheory
	leaves map[SMTLiteral]ast.ASTDependency
	owned  []ast.ASTDependency
}

func (d *smtGrobnerDependencies) close() (result error) {
	for i := len(d.owned) - 1; i >= 0; i-- {
		result = errors.Join(result, d.theory.state.manager.DependencyDec(d.owned[i]))
	}
	d.owned = nil
	d.leaves = nil
	return
}

func (d *smtGrobnerDependencies) retain(value ast.ASTDependency) (ast.ASTDependency, error) {
	if err := d.theory.state.manager.DependencyInc(value); err != nil {
		return ast.ASTDependency{}, err
	}
	d.owned = append(d.owned, value)
	return value, nil
}

func (d *smtGrobnerDependencies) add(root ast.ASTDependency, literal SMTLiteral) (ast.ASTDependency, error) {
	m := d.theory.state.manager
	if d.leaves == nil {
		d.leaves = make(map[SMTLiteral]ast.ASTDependency)
	}
	leaf, found := d.leaves[literal]
	if !found {
		expression, err := d.theory.state.BoolExpression(literal.Var())
		if err != nil {
			return root, err
		}
		if literal.Sign() {
			expression, err = m.MkNot(expression)
			if err != nil {
				return root, err
			}
		}
		leaf, err = m.DependencyLeaf(expression)
		if err != nil {
			return root, err
		}
		leaf, err = d.retain(leaf)
		if err != nil {
			return root, err
		}
		d.leaves[literal] = leaf
	}
	joined, err := m.DependencyJoin(root, leaf)
	if err != nil {
		return root, err
	}
	return d.retain(joined)
}

func (d *smtGrobnerDependencies) boundLiterals(solver *simplex.Solver, column uint32, lower bool) ([]SMTLiteral, error) {
	dependencies, known := solver.BoundDependencies(column, lower)
	var result []SMTLiteral
	if !known {
		for _, assertion := range d.theory.assertions {
			result = append(result, smtLinearAssertionLiteral(&d.theory.atoms[assertion.atom], assertion))
		}
		for _, endpoints := range d.theory.equalityBounds {
			literals, err := NewSMTAntecedentExpander().ExpandEquality(endpoints.lower.left, endpoints.lower.right)
			if err != nil {
				return nil, err
			}
			result = append(result, literals...)
		}
		return result, nil
	}
	for _, dependency := range dependencies {
		literals, err := d.theory.arithmeticDependencyLiterals(dependency)
		if err != nil {
			return nil, err
		}
		result = append(result, literals...)
	}
	return result, nil
}

func (d *smtGrobnerDependencies) addBound(root ast.ASTDependency, solver *simplex.Solver, column uint32, lower bool) (ast.ASTDependency, error) {
	literals, err := d.boundLiterals(solver, column, lower)
	if err != nil {
		return root, err
	}
	for _, literal := range literals {
		var err error
		root, err = d.add(root, literal)
		if err != nil {
			return root, err
		}
	}
	return root, nil
}

func (d *smtGrobnerDependencies) literals(root ast.ASTDependency) ([]SMTLiteral, error) {
	expressions, err := d.theory.state.manager.DependencyLinearize(root, nil)
	if err != nil {
		return nil, err
	}
	result := make([]SMTLiteral, 0, len(expressions))
	for _, expression := range expressions {
		literal, err := d.theory.state.Literal(expression)
		if err != nil {
			return nil, err
		}
		result = append(result, literal)
	}
	return result, nil
}
