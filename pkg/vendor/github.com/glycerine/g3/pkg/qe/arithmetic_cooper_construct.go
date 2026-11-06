package qe

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *ArithmeticCooperRuntime) integerNumeral(value *big.Int) (ast.ASTHandle, error) {
	return r.plugin.manager.ArithmeticIntegerNumeralText(value.String())
}
func (r *ArithmeticCooperRuntime) mod(expression ast.ASTHandle, divisor *big.Rat) (ast.ASTHandle, error) {
	k, err := r.plugin.util.numeralFor(divisor, expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.plugin.manager.MkFamilyApp(r.plugin.family, 13, nil, []ast.ASTHandle{expression, k})
}
func (r *ArithmeticCooperRuntime) zero(expression ast.ASTHandle) (ast.ASTHandle, error) {
	return r.plugin.util.zero(expression)
}
func (r *ArithmeticCooperRuntime) modZero(expression ast.ASTHandle, divisor *big.Rat) (ast.ASTHandle, error) {
	mod, err := r.mod(expression, divisor)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	zero, err := r.zero(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return r.plugin.manager.MkEq(zero, mod)
}
