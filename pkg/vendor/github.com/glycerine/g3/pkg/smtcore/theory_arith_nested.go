package smtcore

// Source: src/smt/theory_arith_nl.h get_polynomial_info, p2expr,
// in_monovariate_monomials, factor, horner, cross_nested, and interval checks.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type smtNestedTerm struct {
	coefficient big.Rat
	expression  ast.ASTHandle
	factors     []ast.ASTHandle
}

type smtNestedVariable struct {
	expression ast.ASTHandle
	count      int
}

type smtNestedArithmetic struct {
	theory       *SMTLinearArithmeticTheory
	solver       *simplex.Solver
	refs         smtArithmeticDivisionCall
	dependencies smtArithmeticBoundDependencyManager
	aliases      map[uint32]ast.ASTHandle
	columns      map[ast.ASTHandle]uint32
}

func (n *smtNestedArithmetic) integer(expression ast.ASTHandle) (bool, error) {
	sort, err := n.theory.state.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	kind, err := n.theory.state.manager.GetDeclKind(sort)
	return kind == 1, err
}

func (n *smtNestedArithmetic) numeral(value *big.Rat, integer bool) (ast.ASTHandle, error) {
	return n.refs.hold(n.theory.state.manager.ArithmeticNumeralRat(value, integer))
}

func (n *smtNestedArithmetic) product(arguments []ast.ASTHandle, integer bool) (ast.ASTHandle, error) {
	if len(arguments) == 0 {
		return n.numeral(big.NewRat(1, 1), integer)
	}
	if len(arguments) == 1 {
		return arguments[0], nil
	}
	return n.refs.app(smtArithOpMul, arguments...)
}

func (n *smtNestedArithmetic) power(variable ast.ASTHandle, degree uint32) (ast.ASTHandle, error) {
	result := variable
	for i := uint32(1); i < degree; i++ {
		var err error
		result, err = n.refs.app(smtArithOpMul, variable, result)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return result, nil
}

func (n *smtNestedArithmetic) column(expression ast.ASTHandle) (uint32, bool) {
	if column, found := n.columns[expression]; found {
		return column, true
	}
	node := n.theory.state.context.enodes[expression]
	if node == nil || !n.theory.IsAttachedToVariable(node) {
		return 0, false
	}
	variable := node.TheoryVar(n.theory.FamilyID())
	if variable < 0 || int(variable) >= len(n.theory.simplexVariables) {
		return 0, false
	}
	return n.theory.simplexVariables[variable], true
}

func (n *smtNestedArithmetic) variables(polynomial []smtNestedTerm) []smtNestedVariable {
	var result []smtNestedVariable
	positions := make(map[ast.ASTHandle]int)
	for _, term := range polynomial {
		seen := make(map[ast.ASTHandle]bool)
		for _, variable := range term.factors {
			if seen[variable] {
				continue
			}
			seen[variable] = true
			column, found := n.column(variable)
			if !found {
				continue
			}
			if _, fixed := smtFixedSimplexColumn(n.solver, column); fixed {
				continue
			}
			if position, exists := positions[variable]; exists {
				result[position].count++
			} else {
				positions[variable] = len(result)
				result = append(result, smtNestedVariable{variable, 1})
			}
		}
	}
	filtered := result[:0]
	for _, variable := range result {
		if variable.count > 1 {
			filtered = append(filtered, variable)
		}
	}
	return filtered
}

func smtNestedDegree(term *smtNestedTerm, variable ast.ASTHandle) uint32 {
	var degree uint32
	for _, factor := range term.factors {
		if factor == variable {
			degree++
		}
	}
	return degree
}

func (n *smtNestedArithmetic) expression(polynomial []smtNestedTerm) (ast.ASTHandle, error) {
	arguments := make([]ast.ASTHandle, 0, len(polynomial))
	for _, term := range polynomial {
		expression := term.expression
		if term.coefficient.Cmp(big.NewRat(1, 1)) != 0 {
			integer, err := n.integer(expression)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if value, numeral, err := n.theory.numeral(expression); err != nil {
				return ast.ASTHandle{}, err
			} else if numeral {
				value.Mul(value, &term.coefficient)
				expression, err = n.numeral(value, integer && value.IsInt())
				if err != nil {
					return ast.ASTHandle{}, err
				}
			} else {
				coefficient, err := n.numeral(&term.coefficient, integer && term.coefficient.IsInt())
				if err != nil {
					return ast.ASTHandle{}, err
				}
				expression, err = n.refs.app(smtArithOpMul, coefficient, expression)
				if err != nil {
					return ast.ASTHandle{}, err
				}
			}
		}
		arguments = append(arguments, expression)
	}
	if len(arguments) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	if len(arguments) == 1 {
		return arguments[0], nil
	}
	return n.refs.app(smtArithOpAdd, arguments...)
}

func (n *smtNestedArithmetic) factor(term smtNestedTerm, variable ast.ASTHandle, degree uint32) (smtNestedTerm, error) {
	if degree == 0 {
		return term, nil
	}
	factors := make([]ast.ASTHandle, 0, len(term.factors))
	removed := uint32(0)
	for _, factor := range term.factors {
		if factor == variable && removed < degree {
			removed++
		} else {
			factors = append(factors, factor)
		}
	}
	if removed != degree {
		return smtNestedTerm{}, ast.ErrASTDomain
	}
	integer, err := n.integer(variable)
	if err != nil {
		return smtNestedTerm{}, err
	}
	expression, err := n.product(factors, integer)
	if err != nil {
		return smtNestedTerm{}, err
	}
	return smtNestedTerm{coefficient: *new(big.Rat).Set(&term.coefficient), expression: expression, factors: factors}, nil
}

func (n *smtNestedArithmetic) horner(polynomial []smtNestedTerm, variable ast.ASTHandle) (ast.ASTHandle, error) {
	degree := ^uint32(0)
	for i := range polynomial {
		if current := smtNestedDegree(&polynomial[i], variable); current < degree {
			degree = current
		}
	}
	var equal, rest []smtNestedTerm
	for _, term := range polynomial {
		current := smtNestedDegree(&term, variable)
		factored, err := n.factor(term, variable, degree)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if current == degree {
			equal = append(equal, factored)
		} else {
			rest = append(rest, factored)
		}
	}
	result, err := n.crossNested(equal, ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if len(rest) != 0 {
		other, err := n.horner(rest, variable)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		result, err = n.refs.app(smtArithOpAdd, other, result)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	if degree != 0 {
		power, err := n.power(variable, degree)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		return n.refs.app(smtArithOpMul, power, result)
	}
	return result, nil
}

func (n *smtNestedArithmetic) crossNested(polynomial []smtNestedTerm, variable ast.ASTHandle) (ast.ASTHandle, error) {
	if variable == (ast.ASTHandle{}) {
		variables := n.variables(polynomial)
		if len(variables) == 0 {
			return n.expression(polynomial)
		}
		selected := variables[0]
		for _, candidate := range variables[1:] {
			if candidate.count > selected.count {
				selected = candidate
			}
		}
		variable = selected.expression
	}
	var indices []int
	monovariate := true
	for i, term := range polynomial {
		degree := smtNestedDegree(&term, variable)
		if degree == 0 {
			continue
		}
		if int(degree) != len(term.factors) {
			monovariate = false
			break
		}
		indices = append(indices, i)
	}
	if monovariate && len(indices) == 2 {
		first, second := &polynomial[indices[0]], &polynomial[indices[1]]
		a, b := new(big.Rat).Set(&first.coefficient), new(big.Rat).Set(&second.coefficient)
		low, high := uint32(len(first.factors)), uint32(len(second.factors))
		if high < low {
			low, high, a, b = high, low, b, a
		}
		difference := high - low
		if difference != 0 && low >= difference && low%2 == difference%2 && b.Sign() != 0 {
			offset := new(big.Rat).Quo(a, new(big.Rat).Mul(big.NewRat(2, 1), b))
			integer, err := n.integer(variable)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if !integer || offset.IsInt() && b.IsInt() {
				power, err := n.power(variable, difference)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				constant, err := n.numeral(offset, integer)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				shift, err := n.refs.app(smtArithOpAdd, constant, power)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				square, err := n.refs.app(smtArithOpMul, shift, shift)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				negativeSquare := new(big.Rat).Neg(new(big.Rat).Mul(offset, offset))
				constant, err = n.numeral(negativeSquare, integer)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				result, err := n.refs.app(smtArithOpAdd, square, constant)
				if err != nil {
					return ast.ASTHandle{}, err
				}
				if low > difference {
					power, err := n.power(variable, low-difference)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					result, err = n.refs.app(smtArithOpMul, power, result)
					if err != nil {
						return ast.ASTHandle{}, err
					}
				}
				if b.Cmp(big.NewRat(1, 1)) != 0 {
					constant, err := n.numeral(b, integer)
					if err != nil {
						return ast.ASTHandle{}, err
					}
					result, err = n.refs.app(smtArithOpMul, constant, result)
					if err != nil {
						return ast.ASTHandle{}, err
					}
				}
				var rest []smtNestedTerm
				for i, term := range polynomial {
					if i != indices[0] && i != indices[1] {
						rest = append(rest, term)
					}
				}
				if len(rest) != 0 {
					other, err := n.crossNested(rest, ast.ASTHandle{})
					if err != nil {
						return ast.ASTHandle{}, err
					}
					result, err = n.refs.app(smtArithOpAdd, result, other)
					if err != nil {
						return ast.ASTHandle{}, err
					}
				}
				return result, nil
			}
		}
	}
	return n.horner(polynomial, variable)
}

func (n *smtNestedArithmetic) evaluate(expression ast.ASTHandle) (*smtOldInterval, error) {
	if column, found := n.column(expression); found {
		return n.theory.nonlinearColumnInterval(n.solver, column, &n.dependencies)
	}
	if value, numeral, err := n.theory.numeral(expression); err != nil {
		return nil, err
	} else if numeral {
		return newSMTOldIntervalPoint(&n.dependencies, value, nil, nil)
	}
	op, own, err := n.theory.operation(expression)
	if err != nil {
		return nil, err
	}
	if !own {
		return newSMTOldInterval(&n.dependencies)
	}
	m := n.theory.state.manager
	count, err := m.GetNumArgs(expression)
	if err != nil {
		return nil, err
	}
	if op == smtArithOpToReal && count == 1 {
		argument, err := m.GetArg(expression, 0)
		if err != nil {
			return nil, err
		}
		return n.evaluate(argument)
	}
	if op != smtArithOpAdd && op != smtArithOpMul {
		return newSMTOldInterval(&n.dependencies)
	}
	initial := new(big.Rat)
	if op == smtArithOpMul {
		initial.SetInt64(1)
	}
	result, err := newSMTOldIntervalPoint(&n.dependencies, initial, nil, nil)
	if err != nil {
		return nil, err
	}
	powers := make(map[ast.ASTHandle]uint32)
	var arguments []ast.ASTHandle
	for i := uint32(0); i < count; i++ {
		argument, err := m.GetArg(expression, i)
		if err != nil {
			return nil, err
		}
		if op == smtArithOpAdd {
			interval, err := n.evaluate(argument)
			if err != nil {
				return nil, err
			}
			if err = result.add(interval); err != nil {
				return nil, err
			}
		} else {
			if powers[argument] == 0 {
				arguments = append(arguments, argument)
			}
			powers[argument]++
		}
	}
	for _, argument := range arguments {
		interval, err := n.evaluate(argument)
		if err != nil {
			return nil, err
		}
		interval.power(powers[argument])
		if err = result.mul(interval); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (n *smtNestedArithmetic) terms(form *smtLinearForm) ([]smtNestedTerm, bool, error) {
	columns := make([]uint32, 0, len(form.terms))
	haveInteger, haveReal := false, false
	for column := range form.terms {
		integer := n.columnInteger(column)
		if integer {
			haveInteger = true
		} else {
			haveReal = true
		}
		columns = append(columns, column)
	}
	coercions, err := n.theory.state.manager.IntRealCoercions()
	if err != nil {
		return nil, false, err
	}
	if haveInteger && haveReal && !coercions {
		return nil, false, nil
	}
	sort.Slice(columns, func(i, j int) bool { return columns[i] < columns[j] })
	scale := big.NewInt(1)
	allInteger := !haveReal
	if allInteger {
		for _, coefficient := range form.terms {
			denominator := coefficient.Denom()
			gcd := new(big.Int).GCD(nil, nil, scale, denominator)
			scale.Mul(new(big.Int).Quo(scale, gcd), denominator)
		}
		denominator := form.constant.Denom()
		gcd := new(big.Int).GCD(nil, nil, scale, denominator)
		scale.Mul(new(big.Int).Quo(scale, gcd), denominator)
	}
	var result []smtNestedTerm
	for _, column := range columns {
		expression := n.aliases[column]
		if variable, found := n.theory.theoryVariable(column); found {
			node, err := n.theory.Enode(variable)
			if err != nil {
				return nil, false, err
			}
			expression = node.owner
		}
		if expression == (ast.ASTHandle{}) {
			return nil, false, ast.ErrASTDomain
		}
		term := smtNestedTerm{expression: expression}
		term.coefficient.Mul(form.terms[column], new(big.Rat).SetInt(scale))
		if factors := n.theory.nonlinearFactors[column]; len(factors) != 0 {
			for _, factor := range factors {
				v, found := n.theory.theoryVariable(factor)
				if !found {
					return nil, false, ast.ErrASTDomain
				}
				node, err := n.theory.Enode(v)
				if err != nil {
					return nil, false, err
				}
				if value, numeral, err := n.theory.numeral(node.owner); err != nil {
					return nil, false, err
				} else if numeral {
					term.coefficient.Mul(&term.coefficient, value)
				} else {
					term.factors = append(term.factors, node.owner)
				}
			}
			integer, err := n.integer(term.expression)
			if err != nil {
				return nil, false, err
			}
			term.expression, err = n.product(term.factors, integer)
			if err != nil {
				return nil, false, err
			}
		} else if value, numeral, err := n.theory.numeral(expression); err != nil {
			return nil, false, err
		} else if numeral {
			term.coefficient.Mul(&term.coefficient, value)
			term.expression, err = n.numeral(big.NewRat(1, 1), allInteger)
			if err != nil {
				return nil, false, err
			}
		} else {
			term.factors = []ast.ASTHandle{expression}
		}
		if term.coefficient.Sign() != 0 {
			result = append(result, term)
		}
	}
	if form.constant.Sign() != 0 {
		one, err := n.numeral(big.NewRat(1, 1), allInteger)
		if err != nil {
			return nil, false, err
		}
		constant := new(big.Rat).Mul(&form.constant, new(big.Rat).SetInt(scale))
		result = append(result, smtNestedTerm{coefficient: *constant, expression: one})
	}
	return result, true, nil
}

func (t *SMTLinearArithmeticTheory) checkCrossNestedConsistency() (conflict bool, result error) {
	solver, status, err := t.buildSimplex()
	if err != nil || status == lbool.Undef {
		return false, err
	}
	if status == lbool.False {
		return true, t.derivedBoundConflict(solver)
	}
	n := smtNestedArithmetic{theory: t, solver: solver, refs: smtArithmeticDivisionCall{theory: t}}
	defer func() { result = errors.Join(result, n.refs.close()) }()
	if conflict, err := n.checkClusterRows(); err != nil || conflict {
		return conflict, err
	}
	for _, assertion := range t.assertions {
		atom := &t.atoms[assertion.atom]
		if !atom.supported || !atom.nonlinear {
			continue
		}
		polynomial, supported, err := n.terms(&atom.form)
		if err != nil {
			return false, err
		}
		if !supported {
			continue
		}
		variables := n.variables(polynomial)
		sort.SliceStable(variables, func(i, j int) bool { return variables[i].count > variables[j].count })
		for _, variable := range variables {
			t.statistics.crossNested++
			interval, err := n.nestedInterval(polynomial, variable.expression)
			if err != nil {
				return false, err
			}
			op := smtLinearEffectiveOp(atom.op, assertion.truth)
			conflict = op == smtArithOpLE && interval.positiveOne() || op == smtArithOpLT && interval.positive() || op == smtArithOpGE && interval.negativeOne() || op == smtArithOpGT && interval.negative()
			if conflict {
				lower := op == smtArithOpLE || op == smtArithOpLT
				return true, t.arithmeticIntervalConflict(interval, lower, smtLinearAssertionLiteral(atom, assertion))
			}
		}
	}
	return false, nil
}
