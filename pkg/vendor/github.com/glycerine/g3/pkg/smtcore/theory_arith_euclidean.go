package smtcore

// Source: src/smt/theory_arith_int.h euclidean_solver_bridge and
// apply_euclidean_solver. Logical replay bounds retain the current SMT scope.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/euclid"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (t *SMTLinearArithmeticTheory) euclideanNumeralOwner() (*numeral.RationalManager, error) {
	if t.euclideanNumerals == nil {
		budget := arena.NewBudget(^uint64(0))
		manager, err := numeral.NewRationalManager(numeral.RationalConfig{Integers: numeral.IntegerConfig{Cells: arena.Config{Budget: budget}, Words: arena.VectorConfig{Descriptors: arena.Config{Budget: budget}}}})
		if err != nil {
			return nil, err
		}
		t.euclideanNumerals = manager
	}
	return t.euclideanNumerals, nil
}

// Source m_es_num_manager belongs to the arithmetic theory. Each temporary
// bridge owns its solver state and borrows this numeral manager.
func (t *SMTLinearArithmeticTheory) Close() error {
	if t == nil || t.euclideanNumerals == nil {
		return nil
	}
	if err := t.euclideanNumerals.CloseChecked(); err != nil {
		return err
	}
	t.euclideanNumerals = nil
	return nil
}

type smtArithmeticExpressionBound struct {
	key         string
	lower       bool
	value       simplex.Infinitesimal
	explanation *smtArithmeticBoundExplanation
}

type smtEuclideanDefinition struct {
	column     uint32
	expression ast.ASTHandle
}

type smtEuclideanBridge struct {
	theory              *SMTLinearArithmeticTheory
	tableau             *simplex.Solver
	solver              *euclid.Solver
	integers            *numeral.IntegerManager
	numbers             []numeral.Int
	variables           map[uint32]uint32
	justifications      []uint32
	normalizationBounds []simplex.ArithmeticBoundUse
	definitions         []smtEuclideanDefinition
	refs                smtArithmeticDivisionCall
}

func (b *smtEuclideanBridge) close() (result error) {
	for i := len(b.numbers) - 1; i >= 0; i-- {
		result = errors.Join(result, b.integers.Del(&b.numbers[i]))
	}
	result = errors.Join(result, b.refs.close(), b.solver.Close())
	return
}

func (b *smtEuclideanBridge) integer(value *big.Int) (numeral.Int, error) {
	index := len(b.numbers)
	b.numbers = append(b.numbers, numeral.NewInt(0))
	err := b.integers.SetString(&b.numbers[index], value.String())
	return b.numbers[index], err
}

func (b *smtEuclideanBridge) bigInteger(value *numeral.Int) (*big.Int, error) {
	text, err := b.integers.String(value)
	if err != nil {
		return nil, err
	}
	result, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return nil, ast.ErrASTDomain
	}
	return result, nil
}

func (b *smtEuclideanBridge) lookup(expression ast.ASTHandle) (uint32, bool) {
	node := b.theory.state.context.enodes[expression]
	if node == nil || !b.theory.IsAttachedToVariable(node) {
		return 0, false
	}
	variable := node.TheoryVar(b.theory.FamilyID())
	if variable < 0 || int(variable) >= len(b.theory.simplexVariables) {
		return 0, false
	}
	return b.theory.simplexVariables[variable], true
}

func (b *smtEuclideanBridge) monomials(expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	op, own, err := b.theory.operation(expression)
	if err != nil {
		return nil, err
	}
	if !own || op != smtArithOpAdd {
		return []ast.ASTHandle{expression}, nil
	}
	m := b.theory.state.manager
	count, err := m.GetNumArgs(expression)
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, count)
	for i := range result {
		result[i], err = m.GetArg(expression, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (b *smtEuclideanBridge) monomial(expression ast.ASTHandle) (*big.Rat, ast.ASTHandle, error) {
	op, own, err := b.theory.operation(expression)
	if err != nil {
		return nil, ast.ASTHandle{}, err
	}
	m := b.theory.state.manager
	if own && op == smtArithOpMul {
		count, err := m.GetNumArgs(expression)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
		if count == 2 {
			first, err := m.GetArg(expression, 0)
			if err != nil {
				return nil, ast.ASTHandle{}, err
			}
			value, matched, err := b.theory.numeral(first)
			if err != nil {
				return nil, ast.ASTHandle{}, err
			}
			if matched {
				second, err := m.GetArg(expression, 1)
				return value, second, err
			}
		}
	}
	return big.NewRat(1, 1), expression, nil
}

func (b *smtEuclideanBridge) assertEquations() error {
	for _, definition := range b.definitions {
		value, fixed := smtFixedSimplexColumn(b.tableau, definition.column)
		if !fixed || !value.IsInt() {
			continue
		}
		if _, matched, err := b.theory.numeral(definition.expression); err != nil {
			return err
		} else if matched {
			continue
		}
		j, err := b.solver.MkJustification()
		if err != nil {
			return err
		}
		for len(b.justifications) <= int(j) {
			b.justifications = append(b.justifications, simplex.NullVariable)
		}
		b.justifications[j] = definition.column
		arguments, err := b.monomials(definition.expression)
		if err != nil {
			return err
		}
		var coefficients []numeral.Int
		var variables []uint32
		usable := true
		for _, argument := range arguments {
			coefficient, pp, err := b.monomial(argument)
			if err != nil {
				return err
			}
			if !coefficient.IsInt() {
				usable = false
				break
			}
			column, found := b.lookup(pp)
			if !found {
				usable = false
				break
			}
			variable, found := b.variables[column]
			if !found {
				variable, err = b.solver.MkVar()
				if err != nil {
					return err
				}
				b.variables[column] = variable
			}
			a, err := b.integer(coefficient.Num())
			if err != nil {
				return err
			}
			coefficients = append(coefficients, a)
			variables = append(variables, variable)
		}
		if usable {
			constant, err := b.integer(new(big.Int).Neg(value.Num()))
			if err != nil {
				return err
			}
			if err = b.solver.AssertEq(coefficients, variables, &constant, j); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *smtEuclideanBridge) congruence(expression ast.ASTHandle) (gcd, constant *big.Int, usable bool, result error) {
	b.normalizationBounds = nil
	arguments, err := b.monomials(expression)
	if err != nil {
		return nil, nil, false, err
	}
	g := new(big.Int)
	missing := false
	var coefficients []numeral.Int
	var variables []uint32
	for _, argument := range arguments {
		coefficient, pp, err := b.monomial(argument)
		if err != nil {
			return nil, nil, false, err
		}
		if !coefficient.IsInt() {
			return nil, nil, false, nil
		}
		column, found := b.lookup(pp)
		variable, registered := b.variables[column]
		if !found || !registered {
			absolute := new(big.Int).Abs(coefficient.Num())
			if missing {
				g.GCD(nil, nil, g, absolute)
			} else {
				g.Set(absolute)
				missing = true
			}
			if g.Cmp(big.NewInt(1)) == 0 {
				return nil, nil, false, nil
			}
		} else {
			a, err := b.integer(coefficient.Num())
			if err != nil {
				return nil, nil, false, err
			}
			coefficients = append(coefficients, a)
			variables = append(variables, variable)
		}
	}
	zero, err := b.integer(new(big.Int))
	if err != nil {
		return nil, nil, false, err
	}
	a, c := numeral.NewInt(0), numeral.NewInt(0)
	defer func() { result = errors.Join(result, b.integers.Del(&a), b.integers.Del(&c)) }()
	var js []uint32
	if err = b.solver.Normalize(coefficients, variables, &zero, &a, &c, &js); err != nil {
		return nil, nil, false, err
	}
	aprime, err := b.bigInteger(&a)
	if err != nil {
		return nil, nil, false, err
	}
	cprime, err := b.bigInteger(&c)
	if err != nil {
		return nil, nil, false, err
	}
	if !missing {
		g.Abs(aprime)
	} else if aprime.Sign() != 0 {
		g.GCD(nil, nil, g, aprime)
	}
	// All returned explanation IDs refer to fixed equations from this call.
	for _, j := range js {
		if int(j) >= len(b.justifications) || b.justifications[j] == simplex.NullVariable {
			return nil, nil, false, ast.ErrASTDomain
		}
		column := b.justifications[j]
		b.normalizationBounds = append(b.normalizationBounds,
			simplex.ArithmeticBoundUse{Column: column, Lower: true},
			simplex.ArithmeticBoundUse{Column: column, Lower: false})
	}
	if g.Cmp(big.NewInt(1)) == 0 {
		return nil, nil, false, nil
	}
	return g, cprime, true, nil
}

func (t *SMTLinearArithmeticTheory) applyEuclideanSolver(tableau *simplex.Solver) (progress bool, result error) {
	manager, err := t.euclideanNumeralOwner()
	if err != nil {
		return false, err
	}
	solver, err := euclid.NewSolver(manager)
	if err != nil {
		return false, err
	}
	m, err := solver.Manager()
	if err != nil {
		return false, errors.Join(err, solver.Close())
	}
	b := smtEuclideanBridge{theory: t, tableau: tableau, solver: solver, integers: m.Integers(), variables: make(map[uint32]uint32), refs: smtArithmeticDivisionCall{theory: t}}
	defer func() { result = errors.Join(result, b.close()) }()
	mask := t.arithmeticIntegerMask()
	for variable, column := range t.simplexVariables {
		if !mask[column] {
			continue
		}
		node, err := t.Enode(SMTTheoryVar(variable))
		if err != nil {
			return false, err
		}
		b.definitions = append(b.definitions, smtEuclideanDefinition{column, node.owner})
	}
	for i := range t.atoms {
		column := t.nextSimplex + uint32(i)
		if !t.atoms[i].supported || !mask[column] || len(tableau.ColumnRows(column)) == 0 {
			continue
		}
		expression, err := t.integerColumnDefinition(&b.refs, tableau, column)
		if err != nil {
			return false, err
		}
		b.definitions = append(b.definitions, smtEuclideanDefinition{column, expression})
	}
	t.statistics.euclideanCalls++
	if err = b.assertEquations(); err != nil {
		return false, err
	}
	if _, err = solver.Solve(); err != nil {
		return false, err
	}
	inconsistent, err := solver.Inconsistent()
	if err != nil {
		return false, err
	}
	// The retained C++ bridge has no conflict-producing path here (TODO).
	if inconsistent {
		return false, nil
	}
	var pending []smtArithmeticReplayBound
	for _, definition := range b.definitions {
		column := definition.column
		if smtArithmeticColumnFixed(tableau, column) {
			continue
		}
		lower, hasLower := tableau.Lower(column)
		upper, hasUpper := tableau.Upper(column)
		if !hasLower && !hasUpper {
			continue
		}
		g, c, usable, err := b.congruence(definition.expression)
		if err != nil {
			return false, err
		}
		if !usable {
			continue
		}
		if g.Sign() == 0 {
			value := simplex.RationalInfinitesimal(new(big.Rat).SetInt(c))
			explanation := smtArithmeticPlainBoundExplanation(tableau, b.normalizationBounds)
			pending = append(pending,
				smtArithmeticReplayBound{column: column, lower: true, value: value.Clone(), explanation: explanation},
				smtArithmeticReplayBound{column: column, lower: false, value: value.Clone(), explanation: explanation})
			continue
		}
		for _, isLower := range []bool{true, false} {
			old, valid := upper, hasUpper
			if isLower {
				old, valid = lower, hasLower
			}
			if !valid {
				continue
			}
			quotient := new(big.Rat).Sub(&old.Main, new(big.Rat).SetInt(c))
			quotient.Quo(quotient, new(big.Rat).SetInt(g))
			integer := floorSMTRational(quotient)
			if isLower && !quotient.IsInt() {
				integer.Add(integer, big.NewInt(1))
			}
			value := new(big.Int).Add(new(big.Int).Mul(g, integer), c)
			bound := simplex.RationalInfinitesimal(new(big.Rat).SetInt(value))
			if isLower && bound.Main.Cmp(&old.Main) > 0 || !isLower && bound.Main.Cmp(&old.Main) < 0 {
				uses := []simplex.ArithmeticBoundUse{{Column: column, Lower: isLower}}
				uses = append(uses, b.normalizationBounds...)
				explanation := smtArithmeticPlainBoundExplanation(tableau, uses)
				pending = append(pending, smtArithmeticReplayBound{column: column, lower: isLower, value: bound, explanation: explanation})
			}
		}
	}
	for _, bound := range pending {
		changed, err := smtTightenReplaySimplexBound(tableau, bound.column, bound.lower, bound.value, bound.explanation)
		if err != nil {
			t.recordArithmeticBoundConflict(tableau, bound.column, bound.lower, bound.explanation)
			return true, t.derivedBoundConflict(tableau)
		}
		if !changed {
			continue
		}
		if _, original := t.theoryVariable(bound.column); original {
			t.derivedBounds = append(t.derivedBounds, bound)
		} else {
			index := bound.column - t.nextSimplex
			if int(index) >= len(t.atoms) {
				return false, ast.ErrASTDomain
			}
			t.expressionBounds = append(t.expressionBounds, smtArithmeticExpressionBound{key: smtArithmeticCoefficientKey(&t.atoms[index].form), lower: bound.lower, value: bound.value.Clone(), explanation: bound.explanation})
		}
		t.statistics.euclideanBounds++
		progress = true
	}
	if progress {
		t.state.theoryProgress++
	}
	return progress, nil
}
