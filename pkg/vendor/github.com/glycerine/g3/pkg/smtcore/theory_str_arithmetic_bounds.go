package smtcore

// Source: src/smt/theory_str.cpp lower_bound/upper_bound and their retained
// theory_mi_arith get_lower/get_upper/to_expr calls in theory_arith_core.h.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// stringArithmeticBound reads the concrete mixed-arithmetic bound service.
// A candidate tableau value is never evidence of a bound. The native
// to_expr conversion refuses infinitesimals; the String caller additionally
// refuses nonintegral rational numerals, including bounds on Real terms.
func (t *SMTStringTheory) stringArithmeticBound(expression ast.ASTHandle, lower bool) (*big.Int, bool, error) {
	family, err := t.state.manager.ArithmeticFamilyID()
	if err != nil {
		return nil, false, err
	}
	arithmetic, mixed := t.state.Theory(family).(*SMTLinearArithmeticTheory)
	node := t.state.context.enodes[expression]
	if !mixed || !arithmetic.mixed || node == nil || arithmetic.solver == nil {
		return nil, false, nil
	}
	variable := node.TheoryVar(family)
	if variable == SMTNullTheoryVar || int(variable) >= len(arithmetic.simplexVariables) {
		return nil, false, nil
	}
	column := arithmetic.simplexVariables[variable]
	bound, exists := arithmetic.solver.Lower(column)
	if !lower {
		bound, exists = arithmetic.solver.Upper(column)
	}
	if _, justified := arithmetic.solver.BoundDependencies(column, lower); exists && !justified {
		// A nonlinear model patch may publish a tableau temporarily fixed to
		// candidate values. Recover the logical endpoint without those fixes;
		// merely refusing it would also lose a weaker, valid source bound.
		logical, status, err := arithmetic.buildSimplex()
		if err != nil || status != lbool.True {
			return nil, false, err
		}
		bound, exists = logical.Lower(column)
		if !lower {
			bound, exists = logical.Upper(column)
		}
	}
	if !exists || bound.Epsilon.Sign() != 0 || !bound.Main.IsInt() {
		return nil, false, nil
	}
	return new(big.Int).Set(bound.Main.Num()), true, nil
}
