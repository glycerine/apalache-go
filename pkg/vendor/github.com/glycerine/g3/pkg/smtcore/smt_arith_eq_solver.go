package smtcore

// Source: src/smt/arith_eq_solver.cpp and src/smt/arith_eq_solver.h retained
// integer-row operations used by the full arithmetic theory.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

// smtArithmeticEquationSolver mutates integer coefficient rows in source
// order. Column zero is the constant and columns one onward are variables.
type smtArithmeticEquationSolver struct{}

func smtIntegerRowValid(row []*big.Int) bool {
	if len(row) < 2 {
		return false
	}
	for _, value := range row {
		if value == nil {
			return false
		}
	}
	return true
}

func smtIntegerAbs(value *big.Int) *big.Int { return new(big.Int).Abs(value) }

func smtIntegerRowGCDNormalize(row []*big.Int) error {
	if !smtIntegerRowValid(row) {
		return ast.ErrASTDomain
	}
	gcd := new(big.Int)
	for _, value := range row {
		if value.Sign() == 0 {
			continue
		}
		if gcd.Sign() == 0 {
			gcd.Abs(value)
		} else {
			gcd.GCD(nil, nil, gcd, smtIntegerAbs(value))
		}
		if gcd.Cmp(big.NewInt(1)) == 0 {
			return nil
		}
	}
	if gcd.Sign() == 0 || gcd.Cmp(big.NewInt(1)) == 0 {
		return nil
	}
	for _, value := range row {
		value.Quo(value, gcd)
	}
	return nil
}

func smtIntegerRowGCDTest(row []*big.Int) (bool, error) {
	if !smtIntegerRowValid(row) {
		return false, ast.ErrASTDomain
	}
	gcd := new(big.Int)
	for index := 1; index < len(row) && gcd.Cmp(big.NewInt(1)) != 0; index++ {
		if row[index].Sign() == 0 {
			continue
		}
		if gcd.Sign() == 0 {
			gcd.Abs(row[index])
		} else {
			gcd.GCD(nil, nil, gcd, smtIntegerAbs(row[index]))
		}
	}
	if gcd.Cmp(big.NewInt(1)) == 0 {
		return true, nil
	}
	if gcd.Sign() == 0 {
		return row[0].Sign() == 0, nil
	}
	return new(big.Int).Rem(row[0], gcd).Sign() == 0, nil
}

func smtIntegerRowAbsMin(row []*big.Int) (int, error) {
	if !smtIntegerRowValid(row) {
		return 0, ast.ErrASTDomain
	}
	index := 1
	minimum := smtIntegerAbs(row[index])
	for current := 2; current < len(row); current++ {
		value := smtIntegerAbs(row[current])
		if minimum.Sign() == 0 || value.Sign() != 0 && value.Cmp(minimum) < 0 {
			index, minimum = current, value
		}
	}
	return index, nil
}

func smtIntegerModHat(value, modulus *big.Int) *big.Int {
	remainder := new(big.Int).Mod(value, modulus)
	if new(big.Int).Lsh(new(big.Int).Set(remainder), 1).Cmp(modulus) > 0 {
		remainder.Sub(remainder, modulus)
	}
	return remainder
}

// SolveEquation performs one source Omega equality-elimination step. On
// success, index identifies the solved variable; fresh reports whether its
// replacement uses a new auxiliary integer variable.
func (*smtArithmeticEquationSolver) SolveEquation(row []*big.Int) (index int, fresh bool, satisfiable bool, err error) {
	if err = smtIntegerRowGCDNormalize(row); err != nil {
		return 0, false, false, err
	}
	if satisfiable, err = smtIntegerRowGCDTest(row); err != nil || !satisfiable {
		return 0, false, satisfiable, err
	}
	index, err = smtIntegerRowAbsMin(row)
	if err != nil {
		return 0, false, false, err
	}
	coefficient := new(big.Int).Set(row[index])
	absolute := smtIntegerAbs(coefficient)
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
		row[position].Set(smtIntegerModHat(row[position], modulus))
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

func smtIntegerRowSubstitute(row, solved []*big.Int, index int) ([]*big.Int, error) {
	if !smtIntegerRowValid(row) || !smtIntegerRowValid(solved) || index < 1 || index >= len(solved) {
		return nil, ast.ErrASTDomain
	}
	if index >= len(row) || row[index].Sign() == 0 {
		return row, nil
	}
	coefficient := new(big.Int).Set(row[index])
	unit := smtIntegerAbs(solved[index]).Cmp(big.NewInt(1)) == 0
	if len(row) < len(solved) {
		for len(row) < len(solved) {
			row = append(row, new(big.Int))
		}
	}
	if unit {
		sign := int64(-1)
		if solved[index].Sign() > 0 {
			sign = 1
		}
		factor := new(big.Int).Mul(coefficient, big.NewInt(sign))
		for position := range solved {
			row[position].Sub(row[position], new(big.Int).Mul(factor, solved[position]))
		}
		return row, nil
	}
	row[index].SetInt64(0)
	for position := range solved {
		row[position].Add(row[position], new(big.Int).Mul(coefficient, solved[position]))
	}
	return row, nil
}

// SolveEquations applies the source GCD combination/elimination algorithm.
// A false result returns an integer row whose constant violates the variable
// coefficient GCD divisibility test.
func (*smtArithmeticEquationSolver) SolveEquations(rows [][]*big.Int) (bool, []*big.Int, error) {
	if len(rows) == 0 {
		return true, nil, nil
	}
	columns := len(rows[0])
	live := make([]int, len(rows))
	for index, row := range rows {
		if !smtIntegerRowValid(row) || len(row) != columns {
			return false, nil, ast.ErrASTDomain
		}
		live[index] = index
		if err := smtIntegerRowGCDNormalize(row); err != nil {
			return false, nil, err
		}
		valid, err := smtIntegerRowGCDTest(row)
		if err != nil {
			return false, nil, err
		}
		if !valid {
			return false, smtIntegerRowCopy(row), nil
		}
	}
	for changed := true; changed && len(live) != 0; {
		changed = false
		for column := 1; column < columns; column++ {
			gcd := new(big.Int)
			var prefixGCD []*big.Int
			var prefixRows []int
			position := 0
			for ; position < len(live); position++ {
				coefficient := rows[live[position]][column]
				if coefficient.Sign() == 0 {
					continue
				}
				if gcd.Sign() == 0 {
					gcd.Abs(coefficient)
				} else {
					gcd.GCD(nil, nil, gcd, smtIntegerAbs(coefficient))
				}
				if gcd.Cmp(big.NewInt(1)) == 0 {
					break
				}
				prefixGCD = append(prefixGCD, new(big.Int).Set(gcd))
				prefixRows = append(prefixRows, live[position])
			}
			if position == len(live) {
				continue
			}
			changed = true
			useful := []int{live[position]}
			gcd.Abs(rows[live[position]][column])
			for cursor := len(prefixGCD); gcd.Cmp(big.NewInt(1)) != 0 && cursor > 0; {
				cursor--
				priorMakesUnit := cursor > 0 && new(big.Int).GCD(nil, nil, gcd, prefixGCD[cursor-1]).Cmp(big.NewInt(1)) == 0
				if cursor == 0 || !priorMakesUnit {
					useful = append(useful, prefixRows[cursor])
					gcd.GCD(nil, nil, gcd, prefixGCD[cursor])
				}
			}
			pivot := rows[useful[0]]
			for _, rowIndex := range useful[1:] {
				other := rows[rowIndex]
				u, v, result := new(big.Int), new(big.Int), new(big.Int)
				result.GCD(u, v, pivot[column], other[column])
				for index := 0; index < columns; index++ {
					pivot[index].Set(new(big.Int).Add(new(big.Int).Mul(u, pivot[index]), new(big.Int).Mul(v, other[index])))
				}
			}
			if smtIntegerAbs(pivot[column]).Cmp(big.NewInt(1)) != 0 {
				return false, nil, nil
			}
			live = append(live[:position], live[position+1:]...)
			for _, rowIndex := range live {
				if rows[rowIndex][column].Sign() == 0 {
					continue
				}
				row, err := smtIntegerRowSubstitute(rows[rowIndex], pivot, column)
				if err != nil {
					return false, nil, err
				}
				rows[rowIndex] = row
				if err = smtIntegerRowGCDNormalize(row); err != nil {
					return false, nil, err
				}
				valid, err := smtIntegerRowGCDTest(row)
				if err != nil {
					return false, nil, err
				}
				if !valid {
					return false, smtIntegerRowCopy(row), nil
				}
			}
		}
	}
	return true, nil, nil
}

func smtIntegerRowCopy(row []*big.Int) []*big.Int {
	copyRow := make([]*big.Int, len(row))
	for index, value := range row {
		copyRow[index] = new(big.Int).Set(value)
	}
	return copyRow
}
