package qe

// Source: src/qe/qe_arith_plugin.cpp div_constraint/bounds_proc divisibility.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// ArithmeticDivisibility owns Term and Atom.
type ArithmeticDivisibility struct {
	manager     *ast.ASTManager
	Term        ast.ASTRef
	Atom        ast.ASTRef
	Coefficient *big.Rat
	Divisor     *big.Rat
	Negated     bool
	Nested      bool
}

func (d *ArithmeticDivisibility) Close() error {
	if d == nil || d.manager == nil {
		return ast.ErrASTHandle
	}
	err := errors.Join(d.manager.DelRef(&d.Atom), d.manager.DelRef(&d.Term))
	if err == nil {
		d.manager = nil
	}
	return err
}
