package qe

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *ArithmeticCooperRuntime) boundedResidue(value ast.ASTHandle, upper *big.Int) (ast.ASTHandle, error) {
	zero, err := r.zero(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	limit, err := r.integerNumeral(upper)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	lower, err := r.plugin.manager.MkArithmeticLE(zero, value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	upperBound, err := r.plugin.manager.MkArithmeticLE(value, limit)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.plugin.manager.MkAnd(lower, upperBound)
}
