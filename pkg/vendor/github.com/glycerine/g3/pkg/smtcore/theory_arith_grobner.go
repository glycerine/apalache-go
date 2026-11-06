package smtcore

// Source: src/smt/theory_arith_nl.h mk_gb_monomial, add_row_to_gb,
// add_monomial_def_to_gb, init_grobner_var_order, and compute_grobner.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/grobner"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func smtFixedSimplexColumn(solver *simplex.Solver, column uint32) (*big.Rat, bool) {
	lower, hasLower := solver.Lower(column)
	upper, hasUpper := solver.Upper(column)
	if !hasLower || !hasUpper || !lower.IsRational() || !upper.IsRational() || lower.Main.Cmp(&upper.Main) != 0 {
		return nil, false
	}
	return new(big.Rat).Set(&lower.Main), true
}

func (t *SMTLinearArithmeticTheory) grobnerMonomial(gb *grobner.Solver, solver *simplex.Solver, aliases map[uint32]ast.ASTHandle, coefficient *big.Rat, column uint32, fixedColumns *[]uint32) (*grobner.Monomial, error) {
	coeff := new(big.Rat).Set(coefficient)
	var variables []ast.ASTHandle
	var factor func(uint32, bool) error
	factor = func(current uint32, expand bool) error {
		if current == 0 {
			return nil // The distinguished constant column is one.
		}
		if expand && len(t.nonlinearFactors[current]) != 0 {
			for _, argument := range t.nonlinearFactors[current] {
				if err := factor(argument, false); err != nil {
					return err
				}
			}
			return nil
		}
		if value, fixed := smtFixedSimplexColumn(solver, current); fixed {
			*fixedColumns = append(*fixedColumns, current)
			coeff.Mul(coeff, value)
			return nil
		}
		// The source preprocessor flattens products. Preserve the same
		// polynomial if a raw Go caller supplies a nested multiplication.
		if len(t.nonlinearFactors[current]) != 0 {
			return factor(current, true)
		}
		expression, found := aliases[current]
		if !found {
			return ast.ErrASTDomain
		}
		if value, numeral, err := t.numeral(expression); err != nil {
			return err
		} else if numeral {
			coeff.Mul(coeff, value)
		} else {
			variables = append(variables, expression)
		}
		return nil
	}
	if err := factor(column, true); err != nil {
		return nil, err
	}
	return gb.NewMonomial(coeff, variables)
}

func (t *SMTLinearArithmeticTheory) assertGrobnerForm(gb *grobner.Solver, solver *simplex.Solver, aliases map[uint32]ast.ASTHandle, form *smtLinearForm, dependencies *smtGrobnerDependencies, premises ...SMTLiteral) (err error) {
	// Grobner rows have rational coefficients. A nonstandard fixed point
	// cannot be projected to its rational part as an asserted equation.
	if form.infinity.Sign() != 0 || form.infinitesimal.Sign() != 0 {
		return nil
	}
	monomials := make([]*grobner.Monomial, 0, len(form.terms)+1)
	var fixedColumns []uint32
	defer func() {
		if err != nil {
			for _, monomial := range monomials {
				err = errors.Join(err, gb.DelMonomial(monomial))
			}
		}
	}()
	columns := make([]uint32, 0, len(form.terms))
	for column := range form.terms {
		columns = append(columns, column)
	}
	sort.Slice(columns, func(i, j int) bool { return columns[i] < columns[j] })
	for _, column := range columns {
		monomial, err := t.grobnerMonomial(gb, solver, aliases, form.terms[column], column, &fixedColumns)
		if err != nil {
			return err
		}
		monomials = append(monomials, monomial)
	}
	if form.constant.Sign() != 0 {
		monomial, err := gb.NewMonomial(&form.constant, nil)
		if err != nil {
			return err
		}
		monomials = append(monomials, monomial)
	}
	var root ast.ASTDependency
	for _, literal := range premises {
		root, err = dependencies.add(root, literal)
		if err != nil {
			return err
		}
	}
	seen := make(map[uint32]bool)
	for _, column := range fixedColumns {
		if seen[column] {
			continue
		}
		seen[column] = true
		for _, lower := range []bool{true, false} {
			root, err = dependencies.addBound(root, solver, column, lower)
			if err != nil {
				return err
			}
		}
	}
	return gb.AssertMonomialsZero(monomials, root)
}

func (t *SMTLinearArithmeticTheory) checkNonlinearGrobner() (conflict bool, err error) {
	if t.nonlinearGBExhausted {
		return false, nil
	}
	// Reconstruct logical assertion bounds before substituting fixed values;
	// temporary candidate-model assignments cannot justify a conflict.
	solver, status, err := t.buildSimplex()
	if err != nil || status == lbool.Undef {
		return false, err
	}
	if status == lbool.False {
		return true, t.derivedBoundConflict(solver)
	}
	cluster, err := t.nonlinearCluster(solver)
	if err != nil {
		return false, err
	}
	refs := smtArithmeticDivisionCall{theory: t}
	defer func() { err = errors.Join(err, refs.close()) }()
	dependencies := smtGrobnerDependencies{theory: t}
	defer func() { err = errors.Join(err, dependencies.close()) }()
	gb, err := grobner.New(t.state.manager)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, gb.Close()) }()
	columns := make(map[ast.ASTHandle]uint32)
	aliases := make(map[uint32]ast.ASTHandle)
	for variable, column := range t.simplexVariables {
		node, err := t.Enode(SMTTheoryVar(variable))
		if err != nil {
			return false, err
		}
		columns[node.owner] = column
		aliases[column] = node.owner
		_, lower := solver.Lower(column)
		_, upper := solver.Upper(column)
		weight := 6 // FREE
		if _, fixed := smtFixedSimplexColumn(solver, column); fixed {
			weight = 0
		} else if lower && upper {
			weight = 2
		} else if lower || upper {
			weight = 4
		}
		if len(t.nonlinearFactors[column]) != 0 {
			weight++ // Source QUOTED_* ordering for product aliases.
		}
		if err = gb.SetWeight(node.owner, weight); err != nil {
			return false, err
		}
	}
	// Replay-only comparison columns have no enode. Local arithmetic
	// aliases preserve their distinct column identities and bounds during
	// polynomial elimination; they never enter the public model or context.
	for _, row := range cluster.rows {
		entries, rowErr := solver.RowEntries(row)
		if rowErr != nil {
			return false, rowErr
		}
		form := newSMTLinearForm()
		for _, entry := range entries {
			column := entry.Variable
			if _, fixed := smtFixedSimplexColumn(solver, column); !fixed {
				if _, exists := aliases[column]; !exists {
					real, sortErr := t.state.manager.MkFamilySort(t.FamilyID(), 0, nil)
					if sortErr != nil {
						return false, sortErr
					}
					alias, aliasErr := refs.hold(t.state.manager.MkVar(column, real))
					if aliasErr != nil {
						return false, aliasErr
					}
					aliases[column] = alias
					columns[alias] = column
					_, lower := solver.Lower(column)
					_, upper := solver.Upper(column)
					weight := 6
					if lower && upper {
						weight = 2
					} else if lower || upper {
						weight = 4
					}
					if weightErr := gb.SetWeight(alias, weight); weightErr != nil {
						return false, weightErr
					}
				}
			}
			form.addTerm(column, entry.Coefficient)
		}
		if rowErr = t.assertGrobnerForm(gb, solver, aliases, &form, &dependencies); rowErr != nil {
			return false, rowErr
		}
	}
	// Definition rows are unconditional equations. Explicit opposite
	// bounds provide the remaining fixed rows in this replay representation.
	definitionVariables := make([]int, 0, len(t.definitions))
	for variable := range t.definitions {
		definitionVariables = append(definitionVariables, int(variable))
	}
	sort.Ints(definitionVariables)
	for _, rawVariable := range definitionVariables {
		variable := SMTTheoryVar(rawVariable)
		definition := t.definitions[variable]
		form := definition.clone()
		form.addTerm(t.simplexVariables[variable], big.NewRat(-1, 1))
		if err = t.assertGrobnerForm(gb, solver, aliases, &form, &dependencies); err != nil {
			return false, err
		}
	}
	for i, first := range t.assertions {
		left, ok := smtLinearNormalizedLE(&t.atoms[first.atom], first)
		if !ok {
			continue
		}
		for _, second := range t.assertions[i+1:] {
			right, ok := smtLinearNormalizedLE(&t.atoms[second.atom], second)
			if ok && smtLinearFormsOpposite(&left, &right) {
				if err = t.assertGrobnerForm(gb, solver, aliases, &left, &dependencies, smtLinearAssertionLiteral(&t.atoms[first.atom], first), smtLinearAssertionLiteral(&t.atoms[second.atom], second)); err != nil {
					return false, err
				}
				break
			}
		}
	}
	for _, column := range t.nonlinearColumns {
		if value, fixed := smtFixedSimplexColumn(solver, column); fixed {
			form := newSMTLinearForm()
			form.addTerm(column, big.NewRat(1, 1))
			form.constant.Neg(value)
			var premises []SMTLiteral
			for _, lower := range []bool{true, false} {
				literals, err := dependencies.boundLiterals(solver, column, lower)
				if err != nil {
					return false, err
				}
				premises = append(premises, literals...)
			}
			if err = t.assertGrobnerForm(gb, solver, aliases, &form, &dependencies, premises...); err != nil {
				return false, err
			}
		}
	}
	nextWeight := 8
	for {
		t.statistics.grobner++
		done, basisErr := gb.ComputeBasis(t.nonlinearGBLimit)
		if basisErr != nil {
			return false, basisErr
		}
		canceled, cancelErr := t.state.manager.Canceled()
		if cancelErr != nil || canceled {
			return false, cancelErr
		}
		if !done {
			t.nonlinearGBExhausted = true
		}
		basisInconsistent := gb.Inconsistent()
		conflict = false
		var manager smtArithmeticBoundDependencyManager
		for _, equation := range gb.Equations() {
			if conflict {
				break
			}
			sum, err := newSMTOldIntervalPoint(&manager, new(big.Rat), nil, nil)
			if err != nil {
				return false, err
			}
			sum, err = t.grobnerSumInterval(solver, columns, &manager, sum, equation.Monomials())
			if err != nil {
				return false, err
			}
			if !sum.containsZero() {
				premises, dependencyErr := dependencies.literals(equation.Dependency())
				if dependencyErr != nil {
					return false, dependencyErr
				}
				return true, t.arithmeticIntervalConflict(sum, sum.positiveOne(), premises...)
			}
			square, squareErr := t.grobnerSquareConflictInterval(solver, columns, &manager, equation.Monomials())
			if squareErr != nil {
				return false, squareErr
			}
			if square != nil {
				premises, dependencyErr := dependencies.literals(equation.Dependency())
				if dependencyErr != nil {
					return false, dependencyErr
				}
				return true, t.arithmeticIntervalConflict(square, square.positiveOne(), premises...)
			}
		}
		conflict = conflict || basisInconsistent
		if conflict || !t.nonlinearGBPerturbate || t.nonlinearGBExhausted {
			break
		}
		modified, orderErr := smtPerturbGrobnerOrder(gb, &nextWeight)
		if orderErr != nil {
			return false, orderErr
		}
		if !modified {
			break
		}
	}
	if conflict {
		literals := make([]SMTLiteral, 0, len(t.assertions))
		for _, assertion := range t.assertions {
			literals = append(literals, smtLinearAssertionLiteral(&t.atoms[assertion.atom], assertion))
		}
		t.recordArithmeticConflict()
		err = t.setArithmeticFallbackConflict(literals)
	}
	return conflict, err
}
