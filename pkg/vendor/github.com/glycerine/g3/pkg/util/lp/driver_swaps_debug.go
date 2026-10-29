//go:build g3debug

package lp

// Source: src/test/lp.cpp:626-668,713-720,3133-3138;
// src/util/lp/sparse_matrix.hpp:66-105.

import (
	"fmt"
	"io"

	z3rand "github.com/glycerine/g3/pkg/util/random"
)

// The source diagnostic setter stores zeros and updates the two directions
// independently. Its two-argument indexed_value leaves m_other indeterminate;
// the retained fill/Get/swap/print consumer never reads that member. Native
// zero Other fields here are not reciprocal-link or elimination metadata.
func setDriverSwapElement(m *FloatSparseMatrix, row, column uint32, value float64) error {
	if row >= m.Dimension() || column >= m.Dimension() {
		return ErrAssertion
	}
	r, err := m.AdjustRow(row)
	if err != nil {
		return err
	}
	c, err := m.AdjustColumn(column)
	if err != nil {
		return err
	}
	if uint64(r) >= uint64(len(m.Rows)) || uint64(c) >= uint64(len(m.Columns)) {
		return ErrBounds
	}
	found := false
	for i := range m.Rows[r] {
		if m.Rows[r][i].Index == c {
			m.Rows[r][i].Value = value
			found = true
			break
		}
	}
	if !found {
		m.Rows[r] = append(m.Rows[r], FloatIndexedValue{Value: value, Index: c})
	}
	found = false
	for i := range m.Columns[c].Values {
		if m.Columns[c].Values[i].Index == r {
			m.Columns[c].Values[i].Value = value
			found = true
			break
		}
	}
	if !found {
		m.Columns[c].Values = append(m.Columns[c].Values, FloatIndexedValue{Value: value, Index: r})
	}
	return nil
}

// SwapTestDriver executes both source debug helpers on the same filled matrix
// and shared random stream. Despite its name, the second helper swaps ROWS,
// while composing its permutation from the right. Preserve its failing source
// equality instead of turning it into a corrected column-swap example.
type SwapTestDriver struct {
	Matrix                                    *FloatSparseMatrix
	Original, Product                         *FloatDenseMatrix
	Permutation                               *FloatPermutation
	Phase, Attempts, Applied, CompletedPhases uint32
	out                                       io.Writer
	started, closed                           bool
}

func NewSwapTestDriver(out io.Writer) *SwapTestDriver {
	return &SwapTestDriver{out: driverReportOutput(out)}
}

func (d *SwapTestDriver) check() error {
	var err error
	d.Product, err = MultiplyFloatDebugMatrices(d.Permutation, d.Matrix)
	if err != nil {
		return err
	}
	equal, err := FloatDebugMatricesEqual(d.Original, d.Product)
	if err != nil {
		return err
	}
	if !equal {
		return ErrAssertion
	}
	d.Product = nil
	return nil
}

func (d *SwapTestDriver) phase(random *z3rand.Generator, right bool) error {
	if err := writeDriverLine(d.out, "testing swaps"); err != nil {
		return err
	}
	var err error
	d.Original, err = CopyFloatDenseMatrix(d.Matrix)
	if err != nil {
		return err
	}
	d.Permutation = NewFloatPermutation(10)
	if err = PrintFloatSparseDebugMatrix(d.Matrix, d.out); err != nil {
		return err
	}
	if err = d.check(); err != nil {
		return err
	}
	for i := 0; i < 100; i++ {
		a, b := random.Next()%10, random.Next()%10
		d.Attempts++
		if a == b {
			continue
		}
		if err = writeDriverLine(d.out, fmt.Sprintf("swap %d %d", a, b)); err != nil {
			return err
		}
		if err = d.Matrix.SwapRows(a, b); err != nil {
			return err
		}
		if right {
			err = d.Permutation.TransposeRight(a, b)
		} else {
			err = d.Permutation.TransposeLeft(a, b)
		}
		if err != nil {
			return err
		}
		d.Applied++
		if err = d.check(); err != nil {
			return err
		}
		if err = PrintFloatSparseDebugMatrix(d.Matrix, d.out); err != nil {
			return err
		}
		if err = writeDriverLine(d.out, ""); err != nil {
			return err
		}
	}
	d.Original, d.Permutation = nil, nil
	d.CompletedPhases++
	return nil
}

func (d *SwapTestDriver) Run(random *z3rand.Generator) error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if random == nil {
		return ErrBounds
	}
	d.Matrix = newFloatSparseMatrix(10)
	v := float64(0)
	for i := int32(9); i >= 0; i-- {
		for j := int32(9); j >= 0; j-- {
			if err := setDriverSwapElement(d.Matrix, uint32(i), uint32(j), v); err != nil {
				return err
			}
			v++
		}
	}
	d.Phase = 1
	if err := d.phase(random, false); err != nil {
		return err
	}
	d.Phase = 2
	if err := d.phase(random, true); err != nil {
		return err
	}
	return d.Close()
}

func (d *SwapTestDriver) Close() error {
	if d.closed {
		return nil
	}
	// All debug matrices own native Go scalar storage. Keep the final sparse
	// matrix available for inspection; release the lexical dense workspaces.
	d.Original, d.Product, d.Permutation = nil, nil, nil
	d.closed = true
	return nil
}
