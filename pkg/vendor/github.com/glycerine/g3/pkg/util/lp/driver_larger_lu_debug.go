//go:build g3debug

package lp

// Source: src/test/lp.cpp:182-224,326-369,372-411,456-500.

import (
	"io"

	z3rand "github.com/glycerine/g3/pkg/util/random"
)

// LargerLUTestDriver executes the deterministic or shared-random 6x12 source
// scenario. Settings and random state are borrowed; all matrix/solver lexical
// owners survive an interrupted operation until Close.
type LargerLUTestDriver struct {
	Matrix          *FloatStaticMatrix
	LU              *FloatLU
	Left, Right     *FloatDenseMatrix
	Workspace       *IndexedFloatVector
	Basis, NBasis   []uint32
	Heading         []int32
	CompletedPivots uint32
	RandomDraws     uint32
	ExitCode        int
	settings        *Settings
	out             io.Writer
	randomized      bool
	started, closed bool
}

func NewLargerLUTestDriver(settings *Settings, out io.Writer, randomized bool) *LargerLUTestDriver {
	return &LargerLUTestDriver{settings: settings, out: driverReportOutput(out), randomized: randomized}
}

func initDriverLUBasis(basis []uint32, columns uint32) ([]int32, []uint32, error) {
	heading := make([]int32, columns)
	for i := range heading {
		heading[i] = -1
	}
	for i, j := range basis {
		if uint64(j) >= uint64(len(heading)) {
			return heading, nil, ErrBounds
		}
		heading[j] = int32(i)
	}
	var nonbasic []uint32
	for j := columns; j > 0; {
		j--
		if heading[j] < 0 {
			nonbasic = append(nonbasic, j)
			heading[j] = -int32(len(nonbasic))
		}
	}
	return heading, nonbasic, nil
}

func (d *LargerLUTestDriver) Run(random *z3rand.Generator) error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	name := " test_larger_lu"
	if d.randomized {
		name = " test_larger_lu_exp"
	}
	if err := writeDriverLine(d.out, name); err != nil {
		return err
	}
	d.Matrix = NewFloatStaticMatrix(6, 12)
	d.Basis = []uint32{1, 3, 0, 4, 5, 6}
	if d.randomized && random == nil {
		return ErrBounds
	}
	for i := uint32(0); i < 6; i++ {
		for j := uint32(0); j < 12; j++ {
			column, value := (j+i)%12, float64(j*j)
			if d.randomized {
				column, value = j, float64(random.Next()%20)
				d.RandomDraws++
			}
			if err := d.Matrix.Set(i, column, value); err != nil {
				return err
			}
		}
	}
	if !d.randomized {
		if err := PrintFloatStaticDebugMatrix(d.Matrix, d.out); err != nil {
			return err
		}
	}
	var err error
	d.Heading, d.NBasis, err = initDriverLUBasis(d.Basis, 12)
	if err != nil {
		return err
	}
	if d.settings == nil {
		return ErrBounds
	}
	d.LU, err = NewFloatLU(d.Matrix, d.Basis, d.settings)
	if err != nil {
		return err
	}
	d.Left, err = d.LU.LeftDebugMatrix(d.Basis)
	if err != nil {
		return err
	}
	d.Right, err = d.LU.RightDebugMatrix()
	if err != nil {
		return err
	}
	equal, err := FloatDebugMatricesEqual(d.Left, d.Right)
	if err != nil {
		return err
	}
	if !equal {
		if d.randomized {
			return ErrAssertion
		}
		if err = writeDriverLine(d.out, "left side"); err != nil {
			return err
		}
		if err = PrintFloatDebugMatrix(d.Left, d.out); err != nil {
			return err
		}
		if err = writeDriverLine(d.out, "right side"); err != nil {
			return err
		}
		if err = PrintFloatDebugMatrix(d.Right, d.out); err != nil {
			return err
		}
		if err = writeDriverLine(d.out, "different sides"); err != nil {
			return err
		}
		if err = writeDriverLine(d.out, "initial factorization is incorrect"); err != nil {
			return err
		}
		// Go callers receive the source exit(1) outcome instead of terminating
		// their process; all reached owners remain available for Close.
		d.ExitCode = 1
		return ErrAssertion
	}
	pivots := [][2]uint32{{9, 0}}
	if d.randomized {
		for i := uint32(0); i < 6; i++ {
			value, err := d.Matrix.Element(i, 8)
			if err != nil {
				return err
			}
			if err = writeDriverLine(d.out, floatLPNumberText(value)); err != nil {
				return err
			}
		}
		pivots = [][2]uint32{{8, 3}, {11, 0}}
	}
	d.Workspace = NewIndexedFloatVector(6)
	for _, pivot := range pivots {
		enter, leave := pivot[0], pivot[1]
		if err = d.LU.PrepareEntering(enter, d.Workspace); err != nil {
			return err
		}
		if err = d.LU.ReplaceColumn(0, d.Workspace, uint32(d.Heading[leave])); err != nil {
			return err
		}
		basic, nonbasic := d.Heading[leave], -d.Heading[enter]-1
		d.Heading[enter], d.Heading[leave] = basic, -nonbasic-1
		d.Basis[basic], d.NBasis[nonbasic] = enter, leave
		d.CompletedPivots++
		equal, err = d.LU.DebugIsCorrect(d.Basis)
		if err != nil {
			return err
		}
		if !equal {
			return ErrAssertion
		}
	}
	return d.Close()
}

func (d *LargerLUTestDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.LU != nil {
		if err := d.LU.Close(); err != nil {
			return err
		}
	}
	d.Left, d.Right, d.Workspace = nil, nil, nil
	d.closed = true
	return nil
}
