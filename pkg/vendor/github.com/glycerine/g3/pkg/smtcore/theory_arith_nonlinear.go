package smtcore

// Source: src/smt/theory_arith_core.h internalize_mul_core and
// src/smt/theory_arith_nl.h get_value, check_monomial_assignment(s).

import (
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Source nonlinear monomials are independent arithmetic variables in the
// linear relaxation. Their defining products are checked before accepting it.
func (t *SMTLinearArithmeticTheory) internalizeNonlinearMonomial(expression ast.ASTHandle) (bool, error) {
	s := t.state
	m := s.manager
	n, err := m.GetNumArgs(expression)
	if err != nil {
		return false, err
	}
	arguments := make([]ast.ASTHandle, n)
	nonNumerals := 0
	for i := range arguments {
		arguments[i], err = m.GetArg(expression, uint32(i))
		if err != nil {
			return false, err
		}
		_, numeral, err := t.numeral(arguments[i])
		if err != nil {
			return false, err
		}
		if !numeral {
			nonNumerals++
		}
	}
	if nonNumerals < 2 {
		return false, nil
	}
	node := s.context.enodes[expression]
	if node == nil {
		node, err = s.MkEnode(expression, false, false, true)
		if err != nil {
			return false, err
		}
	}
	v, err := t.createVariable(node)
	if err != nil {
		return false, err
	}
	column := t.simplexVariables[v]
	if _, found := t.nonlinearFactors[column]; found {
		return true, nil
	}
	factors := make([]uint32, n)
	for i, arg := range arguments {
		factors[i], err = t.ensureVariable(arg)
		if err != nil {
			return false, err
		}
	}
	t.nonlinearColumns = append(t.nonlinearColumns, column)
	t.nonlinearFactors[column] = factors
	return true, nil
}

func (t *SMTLinearArithmeticTheory) nonlinearForm(form *smtLinearForm) bool {
	for column := range form.terms {
		if len(t.nonlinearFactors[column]) != 0 {
			return true
		}
	}
	return false
}

func (t *SMTLinearArithmeticTheory) arithmeticColumnValue(column uint32) *big.Rat {
	value := t.solver.Value(column)
	epsilon := new(big.Rat).Mul(&t.delta, &value.Epsilon)
	return new(big.Rat).Add(&value.Main, epsilon)
}

func (t *SMTLinearArithmeticTheory) checkNonlinearMonomialAssignments() (bool, error) {
	if len(t.nonlinearColumns) == 0 {
		return true, nil
	}
	if err := t.computeModelDelta(); err != nil {
		return false, err
	}
	for _, column := range t.nonlinearColumns {
		variable, found := t.theoryVariable(column)
		if !found {
			return false, ast.ErrASTDomain
		}
		node, err := t.Enode(variable)
		if err != nil {
			return false, err
		}
		if t.state.relevancy != nil && !t.state.relevancy.IsRelevant(node.owner) {
			continue
		}
		product := big.NewRat(1, 1)
		for _, factor := range t.nonlinearFactors[column] {
			product.Mul(product, t.arithmeticColumnValue(factor))
		}
		if product.Cmp(t.arithmeticColumnValue(column)) != 0 {
			return false, nil
		}
	}
	return true, nil
}

// Complete the concrete model of the linear relaxation by evaluating its
// product columns and dependent affine definitions. Replaying every original
// row with these fixed rational values is essential: a relaxed assignment
// alone cannot justify SAT. Failure leaves the original tableau untouched so
// the remaining nonlinear strategies can continue from it.
func (t *SMTLinearArithmeticTheory) completeNonlinearCandidate() (bool, error) {
	if err := t.computeModelDelta(); err != nil {
		return false, err
	}
	values := make(map[uint32]*big.Rat)
	visiting := make(map[uint32]bool)
	var evaluate func(uint32) (*big.Rat, error)
	evaluate = func(column uint32) (*big.Rat, error) {
		if value := values[column]; value != nil {
			return value, nil
		}
		if visiting[column] {
			return nil, ast.ErrASTDomain
		}
		visiting[column] = true
		value := t.arithmeticColumnValue(column)
		if factors := t.nonlinearFactors[column]; len(factors) != 0 {
			value.SetInt64(1)
			for _, factor := range factors {
				argument, err := evaluate(factor)
				if err != nil {
					return nil, err
				}
				value.Mul(value, argument)
			}
		} else if variable, found := t.theoryVariable(column); found {
			if definition, defined := t.definitions[variable]; defined {
				value.Set(&definition.constant)
				for term, coefficient := range definition.terms {
					argument, err := evaluate(term)
					if err != nil {
						return nil, err
					}
					value.Add(value, new(big.Rat).Mul(coefficient, argument))
				}
			}
		}
		delete(visiting, column)
		values[column] = value
		return value, nil
	}
	for _, column := range t.simplexVariables {
		value, err := evaluate(column)
		if err != nil {
			return false, err
		}
		if (t.integer || t.integerColumns[column]) && !value.IsInt() {
			return false, nil
		}
	}
	solver, status, err := t.buildSimplexWithValues(values)
	if err != nil || status != lbool.True {
		return false, err
	}
	t.solver = solver
	return t.checkNonlinearMonomialAssignments()
}

// Source: theory_arith_nl.h mk_interval_for, propagate_nl_upward,
// propagate_nl_downward, and update_bounds_using_interval. Bounds belong to
// the current tableau; rebuilding after a pop recomputes their premises.
func (t *SMTLinearArithmeticTheory) nonlinearColumnInterval(solver *simplex.Solver, column uint32, manager *smtArithmeticBoundDependencyManager) (*smtOldInterval, error) {
	result, err := newSMTOldInterval(manager)
	if err != nil {
		return nil, err
	}
	if lower, ok := solver.Lower(column); ok {
		result.lower = smtFiniteExtended(&lower.Main)
		result.lowerOpen = lower.Epsilon.Sign() > 0
		value := newSMTArithmeticValue(&lower.Main, &lower.Epsilon)
		value.infinity.Set(&lower.Infinity)
		result.lowerDep = manager.Leaf(&smtArithmeticBound{variable: SMTTheoryVar(column), kind: smtArithmeticLowerBound, value: value, replayExplanation: smtArithmeticCopyBoundExplanation(solver, column, true)})
	}
	if upper, ok := solver.Upper(column); ok {
		result.upper = smtFiniteExtended(&upper.Main)
		result.upperOpen = upper.Epsilon.Sign() < 0
		value := newSMTArithmeticValue(&upper.Main, &upper.Epsilon)
		value.infinity.Set(&upper.Infinity)
		result.upperDep = manager.Leaf(&smtArithmeticBound{variable: SMTTheoryVar(column), kind: smtArithmeticUpperBound, value: value, replayExplanation: smtArithmeticCopyBoundExplanation(solver, column, false)})
	}
	return result, nil
}

// Source mul_bound_of charges the resulting accumulator, after raising the
// factor interval to its power. Open and infinite endpoints each cost one;
// closed rational endpoints cost numerator bits plus denominator bits for
// nonintegers. As in source, charging does not itself interrupt this operation.
func (t *SMTLinearArithmeticTheory) multiplyNonlinearColumnBound(solver *simplex.Solver, column, power uint32, target *smtOldInterval) error {
	interval, err := t.nonlinearColumnInterval(solver, column, target.manager)
	if err != nil {
		return err
	}
	interval.power(power)
	if err = target.mul(interval); err != nil {
		return err
	}
	for index := 0; index < 2; index++ {
		endpoint, open := &target.lower, target.lowerOpen
		if index == 1 {
			endpoint, open = &target.upper, target.upperOpen
		}
		cost := uint32(1)
		if !open && !endpoint.infinite() {
			bits := endpoint.value.Num().BitLen()
			if bits == 0 {
				bits = 1
			}
			cost = uint32(bits)
			if !endpoint.value.IsInt() {
				cost += uint32(endpoint.value.Denom().BitLen())
			}
		}
		t.state.manager.Limit.IncOffset(cost)
	}
	return nil
}

func (t *SMTLinearArithmeticTheory) updateNonlinearBounds(solver *simplex.Solver, column uint32, interval *smtOldInterval) (bool, bool) {
	explanations := [2]*smtArithmeticBoundExplanation{
		smtArithmeticIntervalExplanation(interval.manager, interval.lowerDep),
		smtArithmeticIntervalExplanation(interval.manager, interval.upperDep),
	}
	return t.updateArithmeticBoundsWithExplanations(solver, column, interval, explanations)
}

func (t *SMTLinearArithmeticTheory) updateArithmeticBoundsWithExplanations(solver *simplex.Solver, column uint32, interval *smtOldInterval, explanations [2]*smtArithmeticBoundExplanation) (bool, bool) {
	changed := false
	for _, lower := range []bool{true, false} {
		endpoint, open := &interval.upper, interval.upperOpen
		old, exists := solver.Upper(column)
		if lower {
			endpoint, open = &interval.lower, interval.lowerOpen
			old, exists = solver.Lower(column)
		}
		if endpoint.infinite() {
			continue
		}
		bound := simplex.RationalInfinitesimal(&endpoint.value)
		if open {
			if lower {
				bound.Epsilon.SetInt64(1)
			} else {
				bound.Epsilon.SetInt64(-1)
			}
		}
		if t.integer || t.integerColumns[column] {
			integer := floorSMTRational(&bound.Main)
			if lower && (!bound.Main.IsInt() || bound.Epsilon.Sign() > 0) {
				integer.Add(integer, big.NewInt(1))
			} else if !lower && bound.Main.IsInt() && bound.Epsilon.Sign() < 0 {
				integer.Sub(integer, big.NewInt(1))
			}
			bound.Main.SetInt(integer)
			bound.Epsilon.SetInt64(0)
		}
		if exists && (lower && bound.Cmp(&old) <= 0 || !lower && bound.Cmp(&old) >= 0) {
			continue
		}
		index := 1
		if lower {
			index = 0
		}
		_, err := smtTightenReplaySimplexBound(solver, column, lower, bound, explanations[index])
		if err != nil {
			t.recordArithmeticBoundConflict(solver, column, lower, explanations[index])
			return changed, true
		}
		changed = true
	}
	return changed, false
}

// Source arithmetic row bound propagation carries the bounds of nonlinear
// columns through affine aliases, for example x*x >= 0 implies -(x*x) <= 0.
// These are the definition rows retained by the replay tableau.
func (t *SMTLinearArithmeticTheory) propagateArithmeticDefinitionBounds(solver *simplex.Solver) (bool, bool, error) {
	var manager smtArithmeticBoundDependencyManager
	variables := make([]int, 0, len(t.definitions))
	for variable := range t.definitions {
		variables = append(variables, int(variable))
	}
	sort.Ints(variables)
	changed := false
	for _, raw := range variables {
		variable := SMTTheoryVar(raw)
		definition := t.definitions[variable]
		interval, err := newSMTOldIntervalPoint(&manager, &definition.constant, nil, nil)
		if err != nil {
			return false, false, err
		}
		ordered := make([]uint32, 0, len(definition.terms))
		for term := range definition.terms {
			ordered = append(ordered, term)
		}
		sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
		for _, term := range ordered {
			coefficient := definition.terms[term]
			bound, err := t.nonlinearColumnInterval(solver, term, &manager)
			if err != nil {
				return false, false, err
			}
			scalar, err := newSMTOldIntervalPoint(&manager, coefficient, nil, nil)
			if err != nil {
				return false, false, err
			}
			if err = bound.mul(scalar); err != nil {
				return false, false, err
			}
			if err = interval.add(bound); err != nil {
				return false, false, err
			}
		}
		var explanations [2]*smtArithmeticBoundExplanation
		for i, lower := range []bool{true, false} {
			var uses []simplex.ArithmeticBoundUse
			for _, term := range ordered {
				coefficient := definition.terms[term]
				uses = append(uses, simplex.ArithmeticBoundUse{Column: term, Lower: lower == (coefficient.Sign() > 0), Coefficient: *new(big.Rat).Set(coefficient)})
			}
			explanations[i] = smtArithmeticWeightedBoundExplanation(solver, uses)
		}
		progress, conflict := t.updateArithmeticBoundsWithExplanations(solver, t.simplexVariables[variable], interval, explanations)
		if conflict {
			return changed, true, nil
		}
		changed = changed || progress
	}
	return changed, false, nil
}

func (t *SMTLinearArithmeticTheory) propagateNonlinearBounds(solver *simplex.Solver) (bool, bool, error) {
	var manager smtArithmeticBoundDependencyManager
	// Source mk_derived_nl_bound queues facts for propagate_core. Every
	// monomial in this pass must therefore read the bounds present on entry.
	type intervalUpdate struct {
		column   uint32
		interval *smtOldInterval
	}
	var updates []intervalUpdate
	for _, column := range t.nonlinearColumns {
		variable, found := t.theoryVariable(column)
		if !found {
			return false, false, ast.ErrASTDomain
		}
		node, err := t.Enode(variable)
		if err != nil {
			return false, false, err
		}
		if t.state.relevancy != nil && !t.state.relevancy.IsRelevant(node.owner) {
			continue
		}
		factors := t.nonlinearFactors[column]
		powers := make(map[uint32]uint32)
		ordered := make([]uint32, 0, len(factors))
		for _, factor := range factors {
			if powers[factor] == 0 {
				ordered = append(ordered, factor)
			}
			powers[factor]++
		}
		// Source analyze_monomial counts only free odd-power factors.
		// Even powers already have a nonnegative interval without bounds.
		badCount, freeIndex := 0, -1
		for index, factor := range ordered {
			if powers[factor]%2 != 0 && smtArithmeticColumnFree(solver, factor) {
				badCount++
				freeIndex = index
				if badCount >= 2 {
					break
				}
			}
		}
		freeMonomial := smtArithmeticColumnFree(solver, column)
		if badCount >= 2 || badCount == 1 && freeMonomial {
			continue
		}
		if badCount == 0 {
			product, err := newSMTOldIntervalPoint(&manager, big.NewRat(1, 1), nil, nil)
			if err != nil {
				return false, false, err
			}
			for _, factor := range ordered {
				if err = t.multiplyNonlinearColumnBound(solver, factor, powers[factor], product); err != nil {
					return false, false, err
				}
			}
			updates = append(updates, intervalUpdate{column, product})
		}
		// Selection uses the bounds present on entry, before upward
		// propagation. A free monomial receives only the upward pass.
		if freeMonomial {
			continue
		}
		for index, target := range ordered {
			if badCount == 1 && index != freeIndex {
				continue
			}
			if powers[target] != 1 {
				continue // Source interval arithmetic has no root operation.
			}
			others, err := newSMTOldIntervalPoint(&manager, big.NewRat(1, 1), nil, nil)
			if err != nil {
				return false, false, err
			}
			for _, factor := range ordered {
				if factor == target {
					continue
				}
				if err = t.multiplyNonlinearColumnBound(solver, factor, powers[factor], others); err != nil {
					return false, false, err
				}
			}
			if others.containsZero() {
				continue
			}
			interval, err := t.nonlinearColumnInterval(solver, column, &manager)
			if err != nil {
				return false, false, err
			}
			if err = interval.div(others); err != nil {
				return false, false, err
			}
			updates = append(updates, intervalUpdate{target, interval})
		}
	}
	changed := false
	for _, update := range updates {
		progress, conflict := t.updateNonlinearBounds(solver, update.column, update.interval)
		changed = changed || progress
		if conflict {
			return changed, true, nil
		}
	}
	return changed, false, nil
}

func smtArithmeticColumnFree(solver *simplex.Solver, column uint32) bool {
	_, lower := solver.Lower(column)
	_, upper := solver.Upper(column)
	return !lower && !upper
}

// Source: theory_arith_nl.h is_monomial_linear,
// get_monomial_fixed_var_product, and propagate_linear_monomial(s).
// Replaying the resulting equation within this tableau keeps its fixed-bound
// premises local to the current logical SMT scope.
func (t *SMTLinearArithmeticTheory) propagateLinearMonomials(solver *simplex.Solver) (bool, error) {
	var manager smtArithmeticBoundDependencyManager
	for _, column := range t.nonlinearColumns {
		coefficient := big.NewRat(1, 1)
		var remaining uint32
		unfixed := 0
		var fixedBounds []simplex.ArithmeticBoundUse
		for _, factor := range t.nonlinearFactors[column] {
			value, fixed := smtFixedSimplexColumn(solver, factor)
			if fixed {
				if value.Sign() == 0 {
					fixedBounds = nil
				}
				fixedBounds = append(fixedBounds, simplex.ArithmeticBoundUse{Column: factor, Lower: true}, simplex.ArithmeticBoundUse{Column: factor, Lower: false})
				coefficient.Mul(coefficient, value)
				if coefficient.Sign() == 0 {
					break // One zero factor suffices, irrespective of others.
				}
			} else {
				remaining = factor
				unfixed++ // Repeated unfixed factors are distinct occurrences.
			}
		}
		if coefficient.Sign() != 0 && unfixed > 1 {
			continue
		}
		if coefficient.Sign() == 0 || unfixed == 0 {
			explanation := smtArithmeticPlainBoundExplanation(solver, fixedBounds)
			dependency := manager.Leaf(&smtArithmeticBound{replayExplanation: explanation})
			interval, err := newSMTOldIntervalPoint(&manager, coefficient, dependency, dependency)
			if err != nil {
				return false, err
			}
			_, conflict := t.updateNonlinearBounds(solver, column, interval)
			if conflict {
				return true, nil
			}
			continue
		}
		// Source introduces the affine term m-k*x and fixes it at zero.
		// Its fresh base column also preserves the replay tableau's basis
		// when the existing product occurs in earlier comparison rows.
		base := solver.NumVariables()
		variables := []uint32{base, column, remaining}
		coefficients := []big.Rat{*big.NewRat(1, 1), *big.NewRat(-1, 1), *coefficient}
		if _, err := solver.AddRationalRow(base, variables, coefficients); err != nil {
			return false, err
		}
		zero := simplex.RationalInfinitesimal(new(big.Rat))
		explanation := smtArithmeticPlainBoundExplanation(solver, fixedBounds)
		if _, err := smtTightenReplaySimplexBound(solver, base, true, zero, explanation); err != nil {
			return true, nil
		}
		if _, err := smtTightenReplaySimplexBound(solver, base, false, zero, explanation); err != nil {
			return true, nil
		}
	}
	return false, nil
}
