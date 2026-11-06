package realclosure

// Source: src/math/realclosure/mpz_matrix.cpp:38-90 and
// src/math/realclosure/realclosure.cpp:1887-1927.

import "math/big"

// signMatrix is the concrete integer matrix used by real-closure sign
// determination. Cells are independent big.Int values, including after a copy.
type signMatrix struct {
	rows, cols int
	cells      []big.Int
}

func (m *signMatrix) make(rows, cols int) {
	if rows <= 0 || cols <= 0 {
		panic("realclosure: empty sign matrix")
	}
	m.rows, m.cols = rows, cols
	m.cells = make([]big.Int, rows*cols)
}

func (m *signMatrix) at(row, col int) *big.Int {
	return &m.cells[row*m.cols+col]
}

func (m *signMatrix) copyFrom(src *signMatrix) {
	if m == src {
		return
	}
	m.make(src.rows, src.cols)
	for i := range src.cells {
		m.cells[i].Set(&src.cells[i])
	}
}

// tensorProduct follows mpz_matrix_manager::tensor_product. Building the
// result before publication also permits the destination to alias an input.
func (m *signMatrix) tensorProduct(a, b *signMatrix) {
	var result signMatrix
	result.make(a.rows*b.rows, a.cols*b.cols)
	for i := 0; i < result.rows; i++ {
		for j := 0; j < result.cols; j++ {
			result.at(i, j).Mul(a.at(i/b.rows, j/b.cols), b.at(i%b.rows, j%b.cols))
		}
	}
	*m = result
}

// signDetMatrix constructs the sign matrix for a derivative's observed zero,
// positive, and negative root counts. A false result means that its sign
// cannot distinguish roots, as in imp::mk_sign_det_matrix.
func signDetMatrix(equal, positive, negative int) (signMatrix, bool) {
	var m signMatrix
	switch {
	case equal > 0 && positive > 0 && negative == 0:
		m.make(2, 2)
		m.setSmall(1, 1, 0, 1)
	case equal > 0 && positive == 0 && negative > 0:
		m.make(2, 2)
		m.setSmall(1, 1, 0, -1)
	case equal == 0 && positive > 0 && negative > 0:
		m.make(2, 2)
		m.setSmall(1, 1, 1, -1)
	case equal > 0 && positive > 0 && negative > 0:
		m.make(3, 3)
		m.setSmall(1, 1, 1, 0, 1, -1, 0, 1, 1)
	default:
		return m, false
	}
	return m, true
}

func (m *signMatrix) setSmall(values ...int64) {
	for i, v := range values {
		m.cells[i].SetInt64(v)
	}
}
