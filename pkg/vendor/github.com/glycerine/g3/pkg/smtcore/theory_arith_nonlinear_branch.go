package smtcore

// Source: src/smt/theory_arith_nl.h find_nl_var_for_branching and
// branch_nl_int_var. Case splits are Boolean decisions, not asserted units.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (t *SMTLinearArithmeticTheory) nonlinearBranchTarget(bounds *simplex.Solver) (uint32, bool, error) {
	if err := t.computeModelDelta(); err != nil {
		return 0, false, err
	}
	var target uint32
	found, bounded := false, false
	var bestRange big.Rat
	var choices uint32
	for _, column := range t.nonlinearColumns {
		if !t.integer && !t.integerColumns[column] {
			continue
		}
		product := big.NewRat(1, 1)
		for _, factor := range t.nonlinearFactors[column] {
			product.Mul(product, t.arithmeticColumnValue(factor))
		}
		if product.Cmp(t.arithmeticColumnValue(column)) == 0 {
			continue
		}
		for _, factor := range t.nonlinearFactors[column] {
			if !t.integer && !t.integerColumns[factor] {
				continue
			}
			if _, fixed := smtFixedSimplexColumn(bounds, factor); fixed {
				continue
			}
			lower, hasLower := bounds.Lower(factor)
			upper, hasUpper := bounds.Upper(factor)
			if hasLower && hasUpper {
				width := new(big.Rat).Sub(&upper.Main, &lower.Main)
				if !bounded || width.Cmp(&bestRange) < 0 {
					target, found, bounded = factor, true, true
					bestRange.Set(width)
				}
			} else if !bounded {
				choices++
				if t.nonlinearRandom.Bounded(choices) == 0 {
					target, found = factor, true
				}
			}
		}
	}
	return target, found, nil
}

func (t *SMTLinearArithmeticTheory) branchNonlinearIntegerVariable() (branched bool, result error) {
	// Source branch selection uses logical assertion bounds, independently
	// of temporary values tried by nonlinear model completion.
	bounds, status, err := t.buildSimplex()
	if err != nil || status != lbool.True {
		return false, err
	}
	column, found, err := t.nonlinearBranchTarget(bounds)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	variable, found := t.theoryVariable(column)
	if !found {
		return false, ast.ErrASTDomain
	}
	node, err := t.Enode(variable)
	if err != nil {
		return false, err
	}
	m := t.state.manager
	value := new(big.Rat)
	op := int32(-1) // Unbounded variables split at equality with zero.
	if lower, exists := bounds.Lower(column); exists {
		integer := floorSMTRational(&lower.Main)
		if !lower.Main.IsInt() || lower.Epsilon.Sign() > 0 {
			integer.Add(integer, big.NewInt(1))
		}
		value.SetInt(integer)
		op = smtArithOpLE
	} else if upper, exists := bounds.Upper(column); exists {
		integer := floorSMTRational(&upper.Main)
		if upper.Main.IsInt() && upper.Epsilon.Sign() < 0 {
			integer.Sub(integer, big.NewInt(1))
		}
		value.SetInt(integer)
		op = smtArithOpGE
	}
	refs := smtArithmeticDivisionCall{theory: t}
	defer func() { result = errors.Join(result, refs.close()) }()
	numeral, err := refs.hold(m.ArithmeticNumeralRat(value, true))
	if err != nil {
		return false, err
	}
	var bound ast.ASTHandle
	if op < 0 {
		bound, err = refs.hold(m.MkEq(node.owner, numeral))
	} else {
		bound, err = refs.app(op, node.owner, numeral)
	}
	if err != nil {
		return false, err
	}
	if err = t.state.Internalize(bound, true); err != nil {
		return false, err
	}
	if err = t.state.markRelevant(bound); err != nil {
		return false, err
	}
	literal, err := t.state.Literal(bound)
	if err != nil {
		return false, err
	}
	assignment, err := t.state.Assignment(literal)
	if err != nil || assignment != lbool.Undef {
		return false, err
	}
	data, err := t.state.BooleanData(literal.Var())
	if err != nil {
		return false, err
	}
	data.SetTrueFirst()
	t.statistics.nonlinearBranches++
	return true, nil
}
