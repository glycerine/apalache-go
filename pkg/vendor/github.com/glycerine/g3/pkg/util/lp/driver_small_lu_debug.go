//go:build g3debug

package lp

// Source: src/test/lp.cpp:182-224,226-324, retained --test_small_lu.

import (
	"fmt"
	"io"
)

// SmallLUTestDriver borrows settings and owns the literal source matrix,
// factorization and update workspace. Failed report/algorithm prefixes remain
// inspectable until Close; no refactor or replacement matrix is synthesized.
type SmallLUTestDriver struct {
	Matrix          *FloatStaticMatrix
	LU              *FloatLU
	Workspace       *IndexedFloatVector
	Basis, NBasis   []uint32
	Heading         []int32
	PrintedBasis    *FloatDenseMatrix
	Replacements    map[uint32]struct{}
	CompletedPivots uint32
	Extended        bool
	settings        *Settings
	out             io.Writer
	started, closed bool
}

func NewSmallLUTestDriver(settings *Settings, out io.Writer) *SmallLUTestDriver {
	return &SmallLUTestDriver{settings: settings, out: driverReportOutput(out)}
}

func (d *SmallLUTestDriver) check() error {
	ok, err := d.LU.DebugIsCorrect(d.Basis)
	if err != nil {
		return err
	}
	if !ok {
		return ErrAssertion
	}
	return nil
}

func (d *SmallLUTestDriver) Run() error {
	if d.started || d.closed {
		return ErrExactVectorPending
	}
	d.started = true
	if err := writeDriverLine(d.out, " test_small_lu"); err != nil {
		return err
	}
	d.Matrix = NewFloatStaticMatrix(3, 6)
	d.Basis = []uint32{0, 1, 3}
	for _, cell := range []struct {
		i, j uint32
		v    float64
	}{{0, 0, 1}, {0, 2, 3.9}, {2, 3, 11}, {0, 5, -3}, {1, 1, 4}, {1, 4, 7}, {2, 0, 1.8}, {2, 2, 5}, {2, 4, 2}, {2, 5, 8}} {
		if err := d.Matrix.Set(cell.i, cell.j, cell.v); err != nil {
			return err
		}
	}
	if err := PrintFloatStaticDebugMatrix(d.Matrix, d.out); err != nil {
		return err
	}
	d.Heading = []int32{0, 1, -3, 2, -2, -1}
	d.NBasis = []uint32{5, 4, 2} // Source initialization scans columns descending.
	if d.settings == nil {
		return ErrBounds
	}
	var err error
	d.LU, err = NewFloatLU(d.Matrix, d.Basis, d.settings)
	if err != nil {
		return err
	}
	if err = d.check(); err != nil {
		return err
	}
	d.Workspace = NewIndexedFloatVector(d.Matrix.RowCount())
	for step, pivot := range [][2]uint32{{2, 0}, {4, 3}, {5, 1}, {3, 2}} {
		enter, leave := pivot[0], pivot[1]
		if err = writeDriverLine(d.out, fmt.Sprintf("entering %d, leaving %d", enter, leave)); err != nil {
			return err
		}
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
		if step != 0 {
			if err = writeDriverLine(d.out, "we were factoring "); err != nil {
				return err
			}
			d.PrintedBasis, err = d.LU.BasisDebugMatrix(d.Basis)
			if err != nil {
				return err
			}
			// Source takes a base matrix pointer: include its final blank endl.
			if err = PrintFloatDebugMatrix(d.PrintedBasis, d.out); err != nil {
				return err
			}
			d.PrintedBasis = nil
		}
		if err = d.check(); err != nil {
			return err
		}
	}
	d.Matrix.AddRow()
	d.Matrix.AddColumn()
	d.Matrix.AddRow()
	d.Matrix.AddColumn()
	for j := uint32(0); j < d.Matrix.ColumnCount(); j++ {
		if err = d.Matrix.Set(3, j, float64(j)); err != nil {
			return err
		}
		if err = d.Matrix.Set(4, j, float64(j*j)); err != nil {
			return err
		}
	}
	d.Basis = append(d.Basis, d.Matrix.ColumnCount()-2)
	d.Heading = append(d.Heading, int32(len(d.Basis)-1))
	d.Basis = append(d.Basis, d.Matrix.ColumnCount()-1)
	d.Heading = append(d.Heading, int32(len(d.Basis)-1))
	d.Replacements, err = d.LU.ColumnsToReplaceForAddLastRows(d.Heading)
	if err != nil {
		return err
	}
	if err = d.LU.AddLastRowsToB(d.Heading, d.Replacements); err != nil {
		return err
	}
	d.Extended = true
	if err = writeDriverLine(d.out, "here"); err != nil {
		return err
	}
	if err = d.check(); err != nil {
		return err
	}
	return d.Close()
}

func (d *SmallLUTestDriver) Close() error {
	if d.closed {
		return nil
	}
	if d.LU != nil {
		if err := d.LU.Close(); err != nil {
			return err
		}
	}
	d.Workspace, d.PrintedBasis, d.Replacements = nil, nil, nil
	d.closed = true
	return nil
}
