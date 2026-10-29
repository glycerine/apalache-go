package spacer

// Source: src/muz/spacer/spacer_matrix.h and spacer_matrix.cpp.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type spacerMatrix struct {
	rows [][]big.Rat
	cols uint32
}

func newSpacerMatrix(rows, columns uint32) *spacerMatrix {
	result := &spacerMatrix{rows: make([][]big.Rat, rows), cols: columns}
	for i := range result.rows {
		result.rows[i] = make([]big.Rat, columns)
	}
	return result
}

func (m *spacerMatrix) numRows() uint32 { return uint32(len(m.rows)) }
func (m *spacerMatrix) numCols() uint32 { return m.cols }

func (m *spacerMatrix) get(row, column uint32) (*big.Rat, error) {
	if m == nil || row >= uint32(len(m.rows)) || column >= m.cols {
		return nil, ast.ErrASTDomain
	}
	return new(big.Rat).Set(&m.rows[row][column]), nil
}

func (m *spacerMatrix) set(row, column uint32, value *big.Rat) error {
	if m == nil || row >= uint32(len(m.rows)) || column >= m.cols || value == nil {
		return ast.ErrASTDomain
	}
	m.rows[row][column].Set(value)
	return nil
}

// gaussianElimination preserves the source row-selection and subtraction
// order, including its non-absolute maximal-pivot comparison.
func (m *spacerMatrix) gaussianElimination() (uint32, error) {
	if m == nil || len(m.rows) == 0 || m.cols == 0 {
		return 0, nil
	}
	row, column := uint32(0), uint32(0)
	for row < uint32(len(m.rows)) && column < m.cols {
		maxValue := new(big.Rat).Set(&m.rows[row][column])
		maxIndex := row
		for candidate := row + 1; candidate < uint32(len(m.rows)); candidate++ {
			if maxValue.Cmp(&m.rows[candidate][column]) < 0 {
				maxValue.Set(&m.rows[candidate][column])
				maxIndex = candidate
			}
		}
		if maxValue.Sign() == 0 {
			column++
			continue
		}
		if maxIndex != row {
			// This intentionally mirrors the reached source assignment order.
			copy(m.rows[row], m.rows[maxIndex])
			copy(m.rows[maxIndex], m.rows[row])
		}
		pivot := new(big.Rat).Set(&m.rows[row][column])
		if pivot.Cmp(big.NewRat(1, 1)) != 0 {
			for k := range m.rows[row] {
				m.rows[row][k].Quo(&m.rows[row][k], pivot)
			}
		}
		for candidate := uint32(1); candidate < uint32(len(m.rows)); candidate++ {
			if candidate == row {
				continue
			}
			factor := new(big.Rat).Set(&m.rows[candidate][column])
			for k := range m.rows[candidate] {
				product := new(big.Rat).Mul(factor, &m.rows[row][k])
				m.rows[candidate][k].Sub(&m.rows[candidate][k], product)
			}
		}
		row++
		column++
	}
	return row, nil
}

func (m *spacerMatrix) normalize() error {
	if m == nil {
		return ast.ErrASTDomain
	}
	denominator := big.NewInt(1)
	for i := range m.rows {
		for j := range m.rows[i] {
			denominator = lcmBigInt(denominator, m.rows[i][j].Denom())
		}
	}
	factor := new(big.Rat).SetInt(denominator)
	for i := range m.rows {
		for j := range m.rows[i] {
			m.rows[i][j].Mul(&m.rows[i][j], factor)
		}
	}
	return nil
}

func lcmBigInt(left, right *big.Int) *big.Int {
	if left.Sign() == 0 || right.Sign() == 0 {
		return new(big.Int)
	}
	gcd := new(big.Int).GCD(nil, nil, left, right)
	result := new(big.Int).Quo(new(big.Int).Set(left), gcd)
	result.Mul(result, right)
	return result.Abs(result)
}
