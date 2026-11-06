package smtcore

// Source: src/smt/theory_arith_int.h branch_infeasible_int_equality.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
)

func (t *SMTLinearArithmeticTheory) tightIntegerEqualityRows() ([][]*big.Int, []uint32, error) {
	indices := make(map[uint32]int)
	columns := []uint32{simplex.NullVariable}
	var rationalRows [][]big.Rat
	integers := t.arithmeticIntegerMask()
	for _, row := range t.solver.BasicRows() {
		base, err := t.solver.BaseVariable(row)
		if err != nil {
			return nil, nil, err
		}
		value := t.solver.Value(base)
		lower, hasLower := t.solver.Lower(base)
		upper, hasUpper := t.solver.Upper(base)
		minus, plus := value.Clone(), value.Clone()
		one := simplex.RationalInfinitesimal(big.NewRat(1, 1))
		minus.Sub(&one)
		plus.Add(&one)
		var constant big.Rat
		if hasLower && minus.Cmp(&lower) < 0 {
			constant.Set(&lower.Main)
		} else if hasUpper && plus.Cmp(&upper) > 0 {
			constant.Set(&upper.Main)
		} else {
			continue
		}
		entries, err := t.solver.RowEntries(row)
		if err != nil {
			return nil, nil, err
		}
		baseCoefficient, err := t.solver.BaseCoefficient(row)
		if err != nil {
			return nil, nil, err
		}
		coefficients := make([]big.Rat, len(columns))
		usable := true
		for _, entry := range entries {
			column := entry.Variable
			if column == base {
				continue
			}
			coefficient := new(big.Rat).Quo(entry.Coefficient, baseCoefficient)
			if smtArithmeticColumnFixed(t.solver, column) {
				bound, _ := t.solver.Lower(column)
				if bound.Epsilon.Sign() != 0 {
					usable = false
					break
				}
				constant.Add(&constant, new(big.Rat).Mul(coefficient, &bound.Main))
				continue
			}
			if !integers[column] {
				usable = false
				break
			}
			index, found := indices[column]
			if !found {
				index = len(columns)
				indices[column] = index
				columns = append(columns, column)
				coefficients = append(coefficients, big.Rat{})
			}
			coefficients[index].Set(coefficient)
		}
		if !usable {
			continue
		}
		coefficients[0].Set(&constant)
		rationalRows = append(rationalRows, coefficients)
	}
	// With no variable columns the source witness is necessarily trivial
	// and cannot produce a polynomial case split.
	if len(columns) == 1 {
		return nil, columns, nil
	}
	rows := make([][]*big.Int, len(rationalRows))
	for i, row := range rationalRows {
		lcm := big.NewInt(1)
		for j := range row {
			denominator := row[j].Denom()
			gcd := new(big.Int).GCD(nil, nil, lcm, denominator)
			lcm.Mul(new(big.Int).Quo(lcm, gcd), denominator)
		}
		rows[i] = make([]*big.Int, len(columns))
		for j := range rows[i] {
			rows[i][j] = new(big.Int)
			if j < len(row) {
				coefficient := new(big.Rat).Mul(&row[j], new(big.Rat).SetInt(lcm))
				rows[i][j].Set(coefficient.Num())
			}
		}
	}
	return rows, columns, nil
}

func (t *SMTLinearArithmeticTheory) branchIntegerEquality() (progress bool, result error) {
	rows, columns, err := t.tightIntegerEqualityRows()
	if err != nil {
		return false, err
	}
	solver := smtArithmeticEquationSolver{}
	satisfiable, witness, err := solver.SolveEquations(rows)
	if err != nil || satisfiable {
		return false, err
	}
	var terms []simplex.ArithmeticCutTerm
	for i := 1; i < len(witness); i++ {
		if witness[i].Sign() != 0 {
			terms = append(terms, simplex.ArithmeticCutTerm{Column: columns[i], Coefficient: *new(big.Rat).SetInt(witness[i])})
		}
	}
	if len(terms) == 0 {
		return false, nil
	}
	refs := smtArithmeticDivisionCall{theory: t}
	defer func() { result = errors.Join(result, refs.close()) }()
	constant := new(big.Rat).SetInt(witness[0])
	firstBound := new(big.Rat).Sub(big.NewRat(1, 1), constant)
	first, err := t.integerPolynomialGE(&refs, terms, firstBound)
	if err != nil {
		return false, err
	}
	for i := range terms {
		terms[i].Coefficient.Neg(&terms[i].Coefficient)
	}
	secondBound := new(big.Rat).Add(constant, big.NewRat(1, 1))
	second, err := t.integerPolynomialGE(&refs, terms, secondBound)
	if err != nil {
		return false, err
	}
	// Both expressions are pinned before internalization changes the
	// comparison-column range used to reconstruct replay-only expressions.
	for _, bound := range []ast.ASTHandle{first, second} {
		if err = t.state.Internalize(bound, false); err != nil {
			return false, err
		}
		if err = t.state.markRelevant(bound); err != nil {
			return false, err
		}
	}
	left, err := t.state.Literal(first)
	if err != nil {
		return false, err
	}
	right, err := t.state.Literal(second)
	if err != nil {
		return false, err
	}
	if err = t.state.addTheoryAxiom(t.FamilyID(), []SMTLiteral{left, right}); err != nil {
		return false, err
	}
	t.statistics.integerEqualityBranches++
	return true, nil
}

// Source mk_polynomial_ge, shared with mixed-integer Gomory cut emission.
func (t *SMTLinearArithmeticTheory) integerPolynomialGE(refs *smtArithmeticDivisionCall, terms []simplex.ArithmeticCutTerm, bound *big.Rat) (ast.ASTHandle, error) {
	m := t.state.manager
	var arguments []ast.ASTHandle
	allInteger := true
	for _, term := range terms {
		expression, err := t.integerColumnExpression(refs, t.solver, term.Column)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		sort, err := m.GetSort(expression)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		kind, err := m.GetDeclKind(sort)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		allInteger = allInteger && kind == 1 && term.Coefficient.IsInt()
		if term.Coefficient.Cmp(big.NewRat(1, 1)) != 0 {
			numeral, err := refs.hold(m.ArithmeticNumeralRat(&term.Coefficient, kind == 1 && term.Coefficient.IsInt()))
			if err != nil {
				return ast.ASTHandle{}, err
			}
			expression, err = refs.app(smtArithOpMul, numeral, expression)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		arguments = append(arguments, expression)
	}
	if len(arguments) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	polynomial := arguments[0]
	if len(arguments) > 1 {
		var err error
		polynomial, err = refs.app(smtArithOpAdd, arguments...)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	numeral, err := refs.hold(m.ArithmeticNumeralRat(bound, allInteger && bound.IsInt()))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return refs.app(smtArithOpGE, polynomial, numeral)
}
