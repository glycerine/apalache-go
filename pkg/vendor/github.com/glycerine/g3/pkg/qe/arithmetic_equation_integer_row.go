package qe

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func arithmeticEquationGCDNormalize(row []*big.Int) {
	gcd := new(big.Int)
	for _, value := range row {
		if value.Sign() == 0 {
			continue
		}
		if gcd.Sign() == 0 {
			gcd.Abs(value)
		} else {
			gcd.GCD(nil, nil, gcd, new(big.Int).Abs(value))
		}
	}
	if gcd.Sign() == 0 || gcd.Cmp(big.NewInt(1)) == 0 {
		return
	}
	for _, value := range row {
		value.Quo(value, gcd)
	}
}

func arithmeticEquationGCDTest(row []*big.Int) bool {
	gcd := new(big.Int)
	for index := 1; index < len(row) && gcd.Cmp(big.NewInt(1)) != 0; index++ {
		if row[index].Sign() == 0 {
			continue
		}
		if gcd.Sign() == 0 {
			gcd.Abs(row[index])
		} else {
			gcd.GCD(nil, nil, gcd, new(big.Int).Abs(row[index]))
		}
	}
	if gcd.Cmp(big.NewInt(1)) == 0 {
		return true
	}
	if gcd.Sign() == 0 {
		return row[0].Sign() == 0
	}
	return new(big.Int).Rem(row[0], gcd).Sign() == 0
}

func arithmeticEquationModHat(value, modulus *big.Int) *big.Int {
	remainder := new(big.Int).Mod(value, modulus)
	if new(big.Int).Lsh(new(big.Int).Set(remainder), 1).Cmp(modulus) > 0 {
		remainder.Sub(remainder, modulus)
	}
	return remainder
}

func solveArithmeticIntegerRow(row []*big.Int) (index int, fresh, satisfiable bool, err error) {
	if len(row) < 2 {
		return 0, false, false, ast.ErrASTDomain
	}
	arithmeticEquationGCDNormalize(row)
	if !arithmeticEquationGCDTest(row) {
		return 0, false, false, nil
	}
	index = 1
	minimum := new(big.Int).Abs(row[index])
	for current := 2; current < len(row); current++ {
		value := new(big.Int).Abs(row[current])
		if minimum.Sign() == 0 || value.Sign() != 0 && value.Cmp(minimum) < 0 {
			index, minimum = current, value
		}
	}
	coefficient := new(big.Int).Set(row[index])
	absolute := new(big.Int).Abs(coefficient)
	if absolute.Sign() == 0 {
		return index, false, true, nil
	}
	if coefficient.Cmp(big.NewInt(1)) == 0 {
		for _, value := range row {
			value.Neg(value)
		}
	}
	fresh = absolute.Cmp(big.NewInt(1)) != 0
	if !fresh {
		return index, false, true, nil
	}
	modulus := new(big.Int).Add(absolute, big.NewInt(1))
	for position := range row {
		row[position].Set(arithmeticEquationModHat(row[position], modulus))
	}
	if row[index].Cmp(big.NewInt(1)) == 0 {
		for _, value := range row {
			value.Neg(value)
		}
	}
	if row[index].Cmp(big.NewInt(-1)) != 0 {
		return 0, false, false, ast.ErrASTDomain
	}
	row[index].Set(modulus)
	return index, true, true, nil
}
