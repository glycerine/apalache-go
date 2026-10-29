package realclosure

// Source: src/math/realclosure/mpz_matrix.cpp:92-303.

import (
	"math/big"
	"sort"
)

// filterColumns preserves the source's ascending column selection. The
// sign-determination caller retains at least one column.
func (m *signMatrix) filterColumns(src *signMatrix, cols []int) {
	if len(cols) == 0 || len(cols) > src.cols {
		panic("realclosure: invalid sign columns")
	}
	for i, col := range cols {
		if col < 0 || col >= src.cols || i > 0 && col <= cols[i-1] {
			panic("realclosure: sign columns must be strictly ascending")
		}
	}
	if len(cols) == src.cols {
		m.copyFrom(src)
		return
	}
	var selected signMatrix
	selected.make(src.rows, len(cols))
	for i := 0; i < src.rows; i++ {
		for j, col := range cols {
			selected.at(i, j).Set(src.at(i, col))
		}
	}
	*m = selected
}

func (m *signMatrix) swapRows(i, j int) {
	if i == j {
		return
	}
	for col := 0; col < m.cols; col++ {
		m.cells[i*m.cols+col], m.cells[j*m.cols+col] = m.cells[j*m.cols+col], m.cells[i*m.cols+col]
	}
}

// normalizeRow divides by the row gcd. With a right-hand side, a gcd that
// does not divide that value proves there is no integer solution.
func (m *signMatrix) normalizeRow(row int, rhs *big.Int) bool {
	var gcd big.Int
	for col := 0; col < m.cols; col++ {
		cell := m.at(row, col)
		if cell.Sign() == 0 {
			continue
		}
		gcd.GCD(nil, nil, &gcd, cell)
		if gcd.IsInt64() && gcd.Int64() == 1 {
			return true
		}
	}
	if gcd.Sign() == 0 {
		return true
	}
	if rhs != nil {
		var rem big.Int
		rem.Rem(rhs, &gcd)
		if rem.Sign() != 0 {
			return false
		}
		rhs.Quo(rhs, &gcd)
	}
	for col := 0; col < m.cols; col++ {
		m.at(row, col).Quo(m.at(row, col), &gcd)
	}
	return true
}

// eliminate follows the source's integer LCM elimination below a pivot.
func (m *signMatrix) eliminate(pivotRow, pivotCol int, rhs []big.Int) bool {
	pivot := m.at(pivotRow, pivotCol)
	for row := pivotRow + 1; row < m.rows; row++ {
		cell := m.at(row, pivotCol)
		if cell.Sign() == 0 {
			continue
		}
		var gcd, lcm, pivotFactor, rowFactor, a, b big.Int
		gcd.GCD(nil, nil, pivot, cell)
		lcm.Mul(pivot, cell)
		lcm.Abs(&lcm)
		lcm.Quo(&lcm, &gcd)
		pivotFactor.Quo(&lcm, pivot)
		rowFactor.Quo(&lcm, cell)
		for col := pivotCol + 1; col < m.cols; col++ {
			a.Mul(&pivotFactor, m.at(pivotRow, col))
			b.Mul(&rowFactor, m.at(row, col))
			m.at(row, col).Sub(&b, &a)
		}
		if rhs != nil {
			a.Mul(&pivotFactor, &rhs[pivotRow])
			b.Mul(&rowFactor, &rhs[row])
			rhs[row].Sub(&b, &a)
		}
		cell.SetInt64(0)
		if rhs != nil {
			if !m.normalizeRow(row, &rhs[row]) {
				return false
			}
		} else {
			m.normalizeRow(row, nil)
		}
	}
	return true
}

// solveInt is the retained sign-determination integer solver. As in the
// source int-output overload, failure leaves the caller's input untouched.
func (m *signMatrix) solveInt(input []int) ([]int, bool) {
	if m.rows != m.cols || len(input) != m.cols {
		panic("realclosure: sign system must be square")
	}
	var a signMatrix
	a.copyFrom(m)
	rhs := make([]big.Int, m.rows)
	for i, value := range input {
		rhs[i].SetInt64(int64(value))
	}
	for col := 0; col < a.cols; col++ {
		pivot := col
		for pivot < a.rows && a.at(pivot, col).Sign() == 0 {
			pivot++
		}
		if pivot == a.rows {
			return nil, false
		}
		a.swapRows(col, pivot)
		rhs[col], rhs[pivot] = rhs[pivot], rhs[col]
		if !a.eliminate(col, col, rhs) {
			return nil, false
		}
	}
	result := make([]int, a.rows)
	for row := a.rows - 1; row >= 0; row-- {
		pivot := a.at(row, row)
		var remainder big.Int
		remainder.Rem(&rhs[row], pivot)
		if remainder.Sign() != 0 {
			return nil, false
		}
		rhs[row].Quo(&rhs[row], pivot)
		if !rhs[row].IsInt64() || int64(int(rhs[row].Int64())) != rhs[row].Int64() {
			return nil, false
		}
		result[row] = int(rhs[row].Int64())
		for above := 0; above < row; above++ {
			var term big.Int
			term.Mul(a.at(above, row), &rhs[row])
			rhs[above].Sub(&rhs[above], &term)
		}
	}
	return result, true
}

// independentRows selects source rows in original-index priority at each
// pivot, then sorts the selected indices before publishing their values.
func (m *signMatrix) independentRows(src *signMatrix) []int {
	var work signMatrix
	work.copyFrom(src)
	rows := make([]int, work.rows)
	for i := range rows {
		rows[i] = i
	}
	chosen := make([]int, 0, work.cols)
	col := 0
	for row := 0; row < work.rows && col < work.cols; row++ {
		pivot := -1
		for candidate := row; candidate < work.rows; candidate++ {
			if work.at(candidate, col).Sign() != 0 && (pivot < 0 || rows[candidate] < rows[pivot]) {
				pivot = candidate
			}
		}
		if pivot < 0 {
			continue
		}
		work.swapRows(row, pivot)
		rows[row], rows[pivot] = rows[pivot], rows[row]
		chosen = append(chosen, rows[row])
		if len(chosen) == work.cols {
			break
		}
		work.eliminate(row, col, nil)
		col++
	}
	sort.Ints(chosen)
	if len(chosen) == 0 {
		*m = signMatrix{}
		return chosen
	}
	var selected signMatrix
	selected.make(len(chosen), src.cols)
	for row, sourceRow := range chosen {
		for col := 0; col < src.cols; col++ {
			selected.at(row, col).Set(src.at(sourceRow, col))
		}
	}
	*m = selected
	return chosen
}
