package qe

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func cooperLCM(values []*ArithmeticDivisibility) (*big.Int, error) {
	result := big.NewInt(1)
	for _, v := range values {
		if v == nil || v.Divisor == nil || !v.Divisor.IsInt() {
			return nil, ast.ErrASTDomain
		}
		n := new(big.Int).Abs(v.Divisor.Num())
		if n.Sign() == 0 {
			return nil, ast.ErrASTDomain
		}
		g := new(big.Int).GCD(nil, nil, result, n)
		result.Mul(new(big.Int).Quo(result, g), n)
	}
	return result, nil
}

func ArithmeticCooperDivisor(processor *ArithmeticBoundsProcessor) (*big.Int, bool, error) {
	if processor == nil {
		return nil, false, ast.ErrASTHandle
	}
	values := append(processor.Divisibilities(false), processor.Divisibilities(true)...)
	if len(values) == 0 {
		return nil, false, nil
	}
	value, err := cooperLCM(values)
	return value, err == nil, err
}
