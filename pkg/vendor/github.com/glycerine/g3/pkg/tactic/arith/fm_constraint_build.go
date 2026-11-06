package arith

// Source: src/tactic/arith/fm_tactic.cpp:1000-1086.
import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// FMBuildConstraint is the retained add_constraint tail after arithmetic AST
// parsing has produced a concrete linear polynomial.
func FMBuildConstraint(store *FMConstraintStore, variables *FMVariableTable, literals []int32, xs []uint32, coefficients []big.Rat, constant *big.Rat, strict bool, dependency ast.ASTDependency) (*FMConstraint, error) {
	if store == nil || variables == nil || len(xs) != len(coefficients) {
		return nil, ast.ErrASTDomain
	}
	for _, x := range xs {
		if int(x) >= len(variables.Integer) {
			return nil, ast.ErrASTDomain
		}
	}
	c, err := store.New(literals, xs, coefficients, constant, strict, dependency)
	if err != nil {
		return nil, err
	}
	c.MakeInteger()
	if c.Strict && variables.AllInteger(c) {
		c.Strict = false
		c.Constant.Sub(&c.Constant, big.NewRat(1, 1))
	}
	return c, nil
}
