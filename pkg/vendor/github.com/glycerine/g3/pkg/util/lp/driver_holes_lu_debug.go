//go:build g3debug

package lp

// Source: src/test/lp.cpp:413-453, retained test_larger_lu_with_holes.

import "io"

type HolesLUTestDriver struct {
	Matrix                  *FloatStaticMatrix
	LU                      *FloatLU
	Left, Right             *FloatDenseMatrix
	PrintedTail             FloatDebugMatrix // borrowed from LU during its report
	Workspace               *IndexedFloatVector
	Basis, NBasis           []uint32
	Heading                 []int32
	TailCount, PrintedTails uint32
	CompletedPivots         uint32
	InitialMismatch         bool
	settings                *Settings
	out                     io.Writer
	started, closed         bool
}

func NewHolesLUTestDriver(settings *Settings, out io.Writer) *HolesLUTestDriver {
	return &HolesLUTestDriver{settings: settings, out: driverReportOutput(out)}
}

func (d *HolesLUTestDriver) Run() error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if err := writeDriverLine(d.out, " test_larger_lu_with_holes"); err != nil {
		return err
	}
	d.Matrix = NewFloatStaticMatrix(8, 9)
	d.Basis = []uint32{0, 1, 2, 3, 4, 5, 6, 7}
	for _, cell := range []struct {
		i, j uint32
		v    float64
	}{{0, 0, 1}, {0, 1, 2}, {0, 2, 3}, {0, 3, 4}, {0, 4, 5}, {0, 8, 99},
		{1, 1, -6}, {1, 2, 7}, {1, 3, 8}, {1, 4, 9}, {2, 2, 10},
		{3, 2, 11}, {3, 3, -12}, {4, 2, 13}, {4, 3, 14}, {4, 4, 15},
		{5, 4, 28}, {5, 5, -18}, {5, 6, 19}, {5, 7, 25},
		{6, 5, 20}, {6, 6, -21}, {7, 5, 22}, {7, 6, 23}, {7, 7, 24}, {7, 8, 88}} {
		if err := d.Matrix.Set(cell.i, cell.j, cell.v); err != nil {
			return err
		}
	}
	if err := PrintFloatStaticDebugMatrix(d.Matrix, d.out); err != nil {
		return err
	}
	var err error
	d.Heading, d.NBasis, err = initDriverLUBasis(d.Basis, 9)
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
	if err = writeDriverLine(d.out, "printing factorization"); err != nil {
		return err
	}
	d.TailCount = uint32(len(d.LU.Tail))
	for i := len(d.LU.Tail); i > 0; {
		i--
		// The concrete view supplies the source's row/column dimension setters
		// for eta/diagonal tails and the native no-op for full-size views.
		d.PrintedTail, err = d.LU.debugTail(d.LU.Tail[i])
		if err != nil {
			return err
		}
		if err = PrintFloatDebugMatrix(d.PrintedTail, d.out); err != nil {
			return err
		}
		d.PrintedTail = nil
		d.PrintedTails++
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
	d.InitialMismatch = !equal
	if !equal {
		// This source diagnostic continues to the update; it does not exit.
		if err = writeDriverLine(d.out, "different sides"); err != nil {
			return err
		}
	}
	d.Workspace = NewIndexedFloatVector(8)
	if err = d.LU.PrepareEntering(8, d.Workspace); err != nil {
		return err
	}
	if err = d.LU.ReplaceColumn(0, d.Workspace, uint32(d.Heading[0])); err != nil {
		return err
	}
	d.Heading[8], d.Heading[0] = 0, -1
	d.Basis[0], d.NBasis[0] = 8, 0
	d.CompletedPivots++
	equal, err = d.LU.DebugIsCorrect(d.Basis)
	if err != nil {
		return err
	}
	if !equal {
		return ErrAssertion
	}
	return d.Close()
}

func (d *HolesLUTestDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.LU != nil {
		if err := d.LU.Close(); err != nil {
			return err
		}
	}
	d.Left, d.Right, d.PrintedTail, d.Workspace = nil, nil, nil, nil
	d.closed = true
	return nil
}
